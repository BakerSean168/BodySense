"""Assessment reuses stored posture analysis_result (P4)."""

from __future__ import annotations

import pytest

from src.prompts.assessment import format_posture_analysis_section, get_assessment_prompt
from src.services.assessment_service import AssessmentService

_VALID_ASSESSMENT_JSON = (
    '{"health_grade":"B","dimension_scores":'
    '{"posture":70,"exercise":70,"lifestyle":70,'
    '"injury_risk":70,"overall":70},'
    '"identified_issues":[],'
    '"improvement_summary":{"exercise":"x","lifestyle":"y","general":"z"}}'
)

SAMPLE = {
    "has_analysis": True,
    "views": [
        {
            "view": "front",
            "analysis": {
                "overall_confidence": "high",
                "findings": [
                    {
                        "key": "uneven_shoulders",
                        "label": "高低肩倾向",
                        "severity": "mild",
                        "evidence": "右侧肩峰略高",
                    }
                ],
                "summary_markdown": "正面观轻微高低肩。",
            },
        }
    ],
    "findings": [],
    "summaries": ["正面观轻微高低肩。"],
}


def _patch_ai(monkeypatch, captured: dict):
    class _FakeAI:
        async def generate(self, req):
            captured["messages"] = req.messages

            class _Resp:
                text = _VALID_ASSESSMENT_JSON

            return _Resp()

    monkeypatch.setattr(
        "src.services.assessment_service.AIService",
        lambda: _FakeAI(),
    )


def test_format_posture_section_includes_stored_findings():
    section = format_posture_analysis_section(SAMPLE)
    assert "高低肩倾向" in section
    assert "analysis_result" in section
    assert "右侧肩峰" in section


def test_format_posture_section_empty_without_analysis():
    assert format_posture_analysis_section(None) == ""
    assert format_posture_analysis_section({"has_analysis": False}) == ""


def test_assessment_prompt_embeds_posture_analysis():
    prompt = get_assessment_prompt({"age": 30}, posture_analysis=SAMPLE)
    assert "高低肩倾向" in prompt
    assert "已完成的体态照片分析" in prompt


@pytest.mark.asyncio
async def test_assessment_service_text_only_when_analysis_and_no_residual_images(
    monkeypatch,
):
    """Fully covered analysis: no residual images → plain text prompt only."""
    captured: dict = {}
    _patch_ai(monkeypatch, captured)

    result = await AssessmentService().generate_assessment(
        profile={"age": 28},
        images=None,
        posture_analysis=SAMPLE,
    )

    assert result["health_grade"] == "B"
    user_msg = captured["messages"][1]
    assert isinstance(user_msg.content, str)
    assert "高低肩倾向" in user_msg.content


@pytest.mark.asyncio
async def test_assessment_hybrid_keeps_residual_images_with_stored_analysis(
    monkeypatch,
):
    """Go may send completed analysis for some views and raw images for others.

    has_analysis must NOT drop residual images — both surfaces ride together.
    """
    captured: dict = {}
    _patch_ai(monkeypatch, captured)

    residual = "data:image/jpeg;base64,SIDE_VIEW_INCOMPLETE"
    result = await AssessmentService().generate_assessment(
        profile={"age": 28},
        images=[residual],
        posture_analysis=SAMPLE,
    )

    assert result["health_grade"] == "B"
    user_msg = captured["messages"][1]
    # Multimodal content list: text (with stored findings) + residual image.
    assert isinstance(user_msg.content, list)
    text_parts = [b for b in user_msg.content if b.get("type") == "text"]
    image_parts = [b for b in user_msg.content if b.get("type") == "image_url"]
    assert len(text_parts) == 1
    assert "高低肩倾向" in text_parts[0]["text"]
    assert "已完成的体态照片分析" in text_parts[0]["text"]
    assert len(image_parts) == 1
    assert image_parts[0]["image_url"]["url"] == residual


@pytest.mark.asyncio
async def test_assessment_images_only_when_no_stored_analysis(monkeypatch):
    captured: dict = {}
    _patch_ai(monkeypatch, captured)

    result = await AssessmentService().generate_assessment(
        profile={"age": 28},
        images=["data:image/jpeg;base64,AAA"],
        posture_analysis={"has_analysis": False, "views": []},
    )

    assert result["health_grade"] == "B"
    user_msg = captured["messages"][1]
    assert isinstance(user_msg.content, list)
    assert any(b.get("type") == "image_url" for b in user_msg.content)
