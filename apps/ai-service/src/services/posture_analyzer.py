"""Posture photo analysis service (Phase 1 VLM + Phase 2 geometric metrics).

Sends a single-view photo to a vision model and returns a governed, structured
posture analysis. When the optional ``pose`` extra (MediaPipe) is installed,
geometric metrics are computed first and fused into findings. Governance
guarantees:

1. Numeric ``metric`` values may only originate from the pose estimator —
   VLM-invented numbers are stripped (anti-hallucination).
2. Findings are constrained to the keys judgeable from the given view.
3. A medical disclaimer is always present, and red flags trigger a "see a
   professional" message.
"""

from __future__ import annotations

import base64
import json
import logging

from ..ai import AiRequest, AIService
from ..ai.posture_gateway_model import (
    posture_model_settings,
)
from ..ai.types import ChatMessage
from ..configuration.posture_agent_config import (
    PostureAgentManifest,
    get_default_posture_configuration,
    get_posture_configuration,
)
from ..prompts.posture import (
    DEFAULT_DISCLAIMER,
    KEY_LABELS,
    VIEW_ALLOWED_KEYS,
    VIEW_LABEL,
    build_posture_system_prompt,
)
from ..runtime.governance import guard_structured_output
from .pose_estimator import (
    estimate_pose_metrics,
    findings_from_metrics,
    metrics_to_dicts,
)
from .red_flag_detector import get_red_flag_detector

logger = logging.getLogger(__name__)

_REQUIRED_FIELDS = ["view", "findings", "summary_markdown", "disclaimer"]
_VALID_SEVERITY = {"mild", "moderate", "marked"}
_VALID_CONFIDENCE = {"high", "medium", "low"}

# Module-level AIService singleton to avoid re-parsing config on every request.
_ai_service_instance: AIService | None = None


def _get_ai_service() -> AIService:
    global _ai_service_instance
    if _ai_service_instance is None:
        _ai_service_instance = AIService()
    return _ai_service_instance


async def analyze_posture(
    image_bytes: bytes,
    mime_type: str,
    view: str,
    ai: AIService | None = None,
    configuration_id: str | None = None,
) -> dict:
    """Analyze a single posture photo and return a governed result dict."""
    ai = ai or _get_ai_service()

    # North-Star: resolve the exact immutable Agent configuration.
    manifest = get_posture_manifest(configuration_id)

    # Current Posture v2 binds geometric perception into the immutable config.
    # Historical v1 has no pinned mechanism and therefore remains VLM-only.
    mechanism_provenance: dict[str, str] | None = None
    if manifest.geometry_mechanism is not None:
        geo_metrics, mechanism_provenance = estimate_pose_metrics(
            image_bytes,
            view,
            manifest.geometry_mechanism,
        )
    else:
        geo_metrics = []
    geo_findings = findings_from_metrics(geo_metrics)
    metrics_hint = ""
    if geo_metrics:
        lines = [
            f"- {m.name}={m.value}{m.unit} → {m.finding_key}/{m.severity}" for m in geo_metrics
        ]
        metrics_hint = (
            "\n以下数值来自姿态关键点几何计算，请在解释中引用它们，"
            "但不要编造任何未列出的角度或数值：\n" + "\n".join(lines) + "\n"
        )

    b64 = base64.b64encode(image_bytes).decode()
    view_label = VIEW_LABEL.get(view, view)

    messages = [
        ChatMessage(role="system", content=build_posture_system_prompt(view)),
        ChatMessage(
            role="user",
            content=[
                {
                    "type": "text",
                    "text": (
                        f"这是用户的{view_label}站姿照片，"
                        f"请严格按 {view} 视角分析体态并输出规定的 JSON。"
                        f"{metrics_hint}"
                    ),
                },
                {
                    "type": "image_url",
                    "image_url": {"url": f"data:{mime_type};base64,{b64}"},
                },
            ],
        ),
    ]

    # North-Star: pin the exact logical model + generation settings from the
    # immutable manifest so the runtime honors the exact configuration identity.
    resp = await ai.generate(
        AiRequest(
            use_case="posture.analyze",
            messages=messages,
            response_format="json_object",
            temperature=manifest.generation.temperature,
            max_tokens=manifest.generation.max_tokens,
            logical_model=manifest.logical_model,
            model_settings=posture_model_settings(manifest),
        )
    )

    try:
        data = json.loads(resp.text)
    except (json.JSONDecodeError, TypeError):
        logger.warning("posture analysis returned non-JSON output; degrading")
        data = {}

    result = govern_posture_result(
        data,
        view,
        geometric_findings=geo_findings,
        allowed_metrics=metrics_to_dicts(geo_metrics),
    )

    # North-Star: attach the immutable Agent configuration + execution provenance.
    result["agent_configuration"] = manifest.provenance()
    result["execution_provenance"] = {
        "status": "executed",
        "runtime": "single-shot",
        "logical_model": manifest.logical_model,
        "model_group_revision": manifest.model_group_revision,
        "provider": getattr(resp, "provider", ""),
        "model": getattr(resp, "model", ""),
    }
    if mechanism_provenance is not None:
        result["mechanism_provenance"] = mechanism_provenance
    return result


def get_posture_manifest(configuration_id: str | None = None) -> PostureAgentManifest:
    """Resolve the exact immutable Posture Agent configuration."""
    if configuration_id:
        return get_posture_configuration(configuration_id)
    return get_default_posture_configuration()


def govern_posture_result(
    data: dict,
    view: str,
    *,
    geometric_findings: list[dict] | None = None,
    allowed_metrics: list[dict] | None = None,
) -> dict:
    """Apply safety invariants to a raw model result.

    Numeric metrics are allowed only when they match ``allowed_metrics``
    produced by the pose estimator. VLM-invented numbers are stripped.
    """
    if not isinstance(data, dict):
        data = {}

    data["schema_version"] = 1
    data["view"] = view

    allowed = set(VIEW_ALLOWED_KEYS.get(view, []))
    # Index geometric metrics by (name) for anti-hallucination checks.
    allowed_by_name: dict[str, dict] = {}
    for m in allowed_metrics or []:
        if isinstance(m, dict) and m.get("name") is not None:
            allowed_by_name[str(m["name"])] = m

    cleaned_findings: list[dict] = []
    seen_keys: set[str] = set()

    # Prefer geometric findings as the authoritative numeric source.
    for f in geometric_findings or []:
        if not isinstance(f, dict):
            continue
        key = f.get("key")
        if key not in allowed:
            continue
        item = dict(f)
        item["label"] = item.get("label") or KEY_LABELS.get(key, key)
        if item.get("severity") not in _VALID_SEVERITY:
            item["severity"] = "mild"
        if item.get("confidence") not in _VALID_CONFIDENCE:
            item["confidence"] = "low"
        item.setdefault("evidence", "")
        # Keep metric only if it is in the allowed geometric set.
        metric = item.get("metric")
        if isinstance(metric, dict) and metric.get("name") in allowed_by_name:
            src = allowed_by_name[str(metric["name"])]
            item["metric"] = {
                "name": src["name"],
                "value": src["value"],
                "unit": src["unit"],
            }
        else:
            item["metric"] = None
        cleaned_findings.append(item)
        seen_keys.add(str(key))

    for f in data.get("findings", []) or []:
        if not isinstance(f, dict):
            continue
        key = f.get("key")
        # Enforce per-view allow-list: drop cross-view guesses.
        if key not in allowed:
            continue
        # Geometric finding already covers this key — keep qualitative evidence
        # from VLM only when geometric did not already include it.
        if key in seen_keys:
            continue
        item = dict(f)
        # Anti-hallucination: VLM metrics only survive if they match geometry.
        metric = item.get("metric")
        if isinstance(metric, dict) and metric.get("name") in allowed_by_name:
            src = allowed_by_name[str(metric["name"])]
            metric_value = metric.get("value")
            src_value = src.get("value") if isinstance(src, dict) else None
            if metric_value is None or src_value is None:
                item["metric"] = None
            else:
                try:
                    if float(metric_value) == float(src_value):
                        item["metric"] = {
                            "name": src["name"],
                            "value": src["value"],
                            "unit": src["unit"],
                        }
                    else:
                        item["metric"] = None
                except (TypeError, ValueError):
                    item["metric"] = None
        else:
            item["metric"] = None
        # Normalize label / severity / confidence.
        item["label"] = item.get("label") or KEY_LABELS.get(key, key)
        if item.get("severity") not in _VALID_SEVERITY:
            item["severity"] = "mild"
        if item.get("confidence") not in _VALID_CONFIDENCE:
            item["confidence"] = "low"
        item.setdefault("evidence", "")
        cleaned_findings.append(item)
        seen_keys.add(str(key))
    data["findings"] = cleaned_findings
    if allowed_metrics:
        data["geometric_metrics"] = list(allowed_metrics)

    # Always carry a disclaimer.
    if not data.get("disclaimer"):
        data["disclaimer"] = DEFAULT_DISCLAIMER

    data.setdefault("summary_markdown", "")
    data.setdefault("overall_confidence", "medium")
    data.setdefault("red_flags", [])

    # Red-flag scan over the summary + evidence text (high-recall, reuses the
    # consultation detector). Merge with any model-declared red flags.
    scan_text = (
        data.get("summary_markdown", "")
        + " "
        + " ".join(f.get("evidence", "") for f in cleaned_findings)
    )
    rf = get_red_flag_detector().detect([], scan_text)
    if rf.has_red_flags:
        existing = {
            (r.get("category"), r.get("message")) for r in data["red_flags"] if isinstance(r, dict)
        }
        for flag in rf.flags:
            item = {"category": flag.category, "message": flag.message}
            if (item["category"], item["message"]) not in existing:
                data["red_flags"].append(item)

    # Single governance seam (P2 Phase C): same entry as diagnosis/treatment.
    guarded = guard_structured_output("posture", data)
    if guarded.verdict == "rejected":
        # Block raw model content; return a minimal safe shell for the job path.
        return {
            "schema_version": 1,
            "view": view,
            "overall_confidence": "low",
            "findings": [],
            "red_flags": [],
            "summary_markdown": guarded.safety_fallback
            or "体态分析未通过安全校验，请重新上传或咨询专业人士。",
            "disclaimer": data.get("disclaimer") or DEFAULT_DISCLAIMER,
            "governance": {
                "verdict": "rejected",
                "kind": "posture",
                "reasons": list(guarded.reasons),
                "issues": list(guarded.issues),
            },
            "safety_fallback": guarded.safety_fallback,
        }

    out = dict(guarded.payload or data)
    if guarded.verdict == "degraded":
        out["overall_confidence"] = "low"
        out.setdefault(
            "safety_note",
            "输出已通过治理但置信度降低，请结合专业意见谨慎参考。",
        )
    out["governance"] = {
        "verdict": guarded.verdict,
        "kind": "posture",
        "reasons": list(guarded.reasons),
        "issues": list(guarded.issues),
    }
    return out
