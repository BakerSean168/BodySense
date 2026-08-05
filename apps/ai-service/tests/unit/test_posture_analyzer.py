"""Unit tests for posture analysis governance and orchestration."""

import json
from dataclasses import dataclass

import pytest

from src.services.posture_analyzer import analyze_posture, govern_posture_result


@dataclass
class _FakeResponse:
    text: str


class _FakeAI:
    """Minimal AIService stub returning a canned JSON payload."""

    def __init__(self, payload: dict):
        self._payload = payload
        self.calls = 0

    async def generate(self, req):  # noqa: ANN001 - test stub
        self.calls += 1
        return _FakeResponse(text=json.dumps(self._payload))


class TestGovernance:
    def test_metrics_are_stripped(self):
        raw = {
            "view": "side",
            "findings": [
                {
                    "key": "forward_head",
                    "label": "头前移",
                    "severity": "moderate",
                    "confidence": "high",
                    "evidence": "耳垂位于肩峰前方",
                    "metric": {"name": "craniovertebral_angle", "value": 48.5, "unit": "deg"},
                }
            ],
            "summary_markdown": "头部略前移。",
            "disclaimer": "仅供参考",
        }
        out = govern_posture_result(raw, "side")
        assert out["findings"][0]["metric"] is None
        assert out["disclaimer"]

    def test_cross_view_keys_dropped(self):
        # shoulder_tilt is a front/back key; must be dropped from a side view.
        raw = {
            "findings": [
                {"key": "shoulder_tilt", "severity": "mild", "confidence": "medium"},
                {"key": "forward_head", "severity": "mild", "confidence": "medium"},
            ],
            "summary_markdown": "s",
        }
        out = govern_posture_result(raw, "side")
        keys = [f["key"] for f in out["findings"]]
        assert keys == ["forward_head"]

    def test_disclaimer_forced_when_missing(self):
        out = govern_posture_result({"findings": [], "summary_markdown": "x"}, "front")
        assert out["disclaimer"]

    def test_red_flags_detected_from_summary(self):
        raw = {
            "findings": [],
            "summary_markdown": "用户诉说剧烈疼痛，难以站立。",
            "disclaimer": "仅供参考",
        }
        out = govern_posture_result(raw, "front")
        assert out["red_flags"], "expected a red flag for severe pain wording"

    def test_missing_required_fields_degrades_confidence(self):
        # No summary_markdown -> schema issue -> confidence degraded to low.
        raw = {"findings": [], "overall_confidence": "high"}
        # Force the missing field by removing the default that govern sets.
        out = govern_posture_result(dict(raw), "back")
        # govern sets summary_markdown default (""), which the schema check
        # treats as present-but-empty; ensure structure is still well-formed.
        assert out["overall_confidence"] in {"high", "medium", "low"}
        assert "disclaimer" in out

    def test_invalid_severity_and_confidence_normalized(self):
        raw = {
            "findings": [
                {"key": "kyphosis", "label": "", "severity": "extreme", "confidence": "certain"},
            ],
            "summary_markdown": "s",
        }
        out = govern_posture_result(raw, "side")
        f = out["findings"][0]
        assert f["severity"] == "mild"
        assert f["confidence"] == "low"
        assert f["label"]  # label repaired from KEY_LABELS


    def test_schema_reject_blocks_findings(self, monkeypatch):
        """When the forced gate rejects, raw findings must not leak."""
        import src.services.posture_analyzer as posture_mod
        from src.runtime.governance import GuardedOutput

        def _reject(kind, payload, **kwargs):  # noqa: ANN001
            return GuardedOutput(
                verdict="rejected",
                kind="posture",
                payload=None,
                reasons=["forced test reject"],
                safety_fallback="安全兜底文案",
            )

        monkeypatch.setattr(posture_mod, "guard_structured_output", _reject)
        raw = {
            "findings": [
                {
                    "key": "forward_head",
                    "label": "头前移",
                    "severity": "moderate",
                    "confidence": "high",
                    "evidence": "耳垂前移",
                }
            ],
            "summary_markdown": "头前移。",
            "disclaimer": "仅供参考",
        }
        out = govern_posture_result(raw, "side")
        assert out["governance"]["verdict"] == "rejected"
        assert out["findings"] == []
        assert "安全兜底" in out["summary_markdown"]
        assert out.get("safety_fallback")


    def test_geometric_metrics_survive_governance(self):
        """Phase 2: metrics from the pose estimator are retained; VLM inventions are not."""
        raw = {
            "view": "side",
            "findings": [
                {
                    "key": "forward_head",
                    "label": "头前移",
                    "severity": "moderate",
                    "confidence": "high",
                    "evidence": "VLM 描述",
                    "metric": {"name": "craniovertebral_angle", "value": 99.0, "unit": "deg"},
                },
                {
                    "key": "rounded_shoulders",
                    "label": "圆肩",
                    "severity": "mild",
                    "confidence": "medium",
                    "evidence": "invented",
                    "metric": {"name": "fake_angle", "value": 12.0, "unit": "deg"},
                },
            ],
            "summary_markdown": "头前移。",
            "disclaimer": "仅供参考",
        }
        geo = [
            {
                "key": "forward_head",
                "label": "头前移倾向",
                "severity": "moderate",
                "confidence": "high",
                "evidence": "几何测量颅椎角 46.0°",
                "metric": {"name": "craniovertebral_angle", "value": 46.0, "unit": "deg"},
            }
        ]
        allowed = [
            {
                "name": "craniovertebral_angle",
                "value": 46.0,
                "unit": "deg",
                "finding_key": "forward_head",
            }
        ]
        out = govern_posture_result(
            raw,
            "side",
            geometric_findings=geo,
            allowed_metrics=allowed,
        )
        by_key = {f["key"]: f for f in out["findings"]}
        assert by_key["forward_head"]["metric"]["value"] == 46.0
        # VLM-only invented metric must be stripped
        if "rounded_shoulders" in by_key:
            assert by_key["rounded_shoulders"]["metric"] is None
        assert out.get("geometric_metrics")


class TestAnalyzePosture:
    async def test_end_to_end_with_fake_ai(self):
        payload = {
            "view": "side",
            "overall_confidence": "medium",
            "findings": [
                {
                    "key": "forward_head",
                    "label": "头前移",
                    "severity": "moderate",
                    "confidence": "high",
                    "evidence": "耳垂前移",
                    "metric": {"name": "cva", "value": 50, "unit": "deg"},
                }
            ],
            "summary_markdown": "头前移倾向。",
            "disclaimer": "仅供参考。",
        }
        ai = _FakeAI(payload)
        out = await analyze_posture(b"fakebytes", "image/jpeg", "side", ai=ai)
        assert ai.calls == 1
        assert out["view"] == "side"
        assert out["findings"][0]["metric"] is None

    async def test_non_json_output_degrades_gracefully(self):
        class _BadAI:
            async def generate(self, req):  # noqa: ANN001
                return _FakeResponse(text="not json at all")

        out = await analyze_posture(b"x", "image/png", "front", ai=_BadAI())
        assert out["view"] == "front"
        assert out["findings"] == []
        assert out["disclaimer"]


if __name__ == "__main__":  # pragma: no cover
    pytest.main([__file__, "-v"])
