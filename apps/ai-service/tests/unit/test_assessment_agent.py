"""Typed Assessment Agent boundary tests."""

from __future__ import annotations

import base64

import pytest
from pydantic_ai.models.test import TestModel

from src.agents.assessment_agent import create_assessment_agent
from src.models.assessment import AssessmentAgentOutput, AssessmentDependencies
from src.services.assessment_service import AssessmentService


def _output() -> dict:
    return {
        "status": "completed",
        "health_grade": "B",
        "dimension_scores": {
            "posture": 72,
            "exercise": 68,
            "lifestyle": 70,
            "injury_risk": 75,
            "overall": 71,
        },
        "observations": [
            {
                "kind": "posture_alignment",
                "body_region": "肩部",
                "label": "高低肩倾向",
                "description": "正面图中右侧肩峰略高。",
                "severity": "轻度",
                "confidence": "中",
                "method": "posture_photo_front",
                "condition": {"view": "front"},
            }
        ],
        "summary": "当前资料支持一项待审核的体态观察。",
        "information_gaps": [],
        "safety_notes": [],
    }


@pytest.mark.asyncio
async def test_assessment_agent_returns_typed_observations_without_advice() -> None:
    model = TestModel(custom_output_args=_output())
    agent = create_assessment_agent(model)
    result = await agent.run(
        "Create an observation-only report.",
        deps=AssessmentDependencies(profile={"age": 30}),
    )

    assert isinstance(result.output, AssessmentAgentOutput)
    assert result.output.observations[0].label == "高低肩倾向"
    dumped = result.output.model_dump(mode="json")
    assert "improvement_summary" not in dumped
    assert "treatment" not in dumped


@pytest.mark.asyncio
async def test_assessment_service_supports_typed_multimodal_content() -> None:
    model = TestModel(custom_output_args=_output())
    service = AssessmentService(assessment_agent=create_assessment_agent(model))
    image = "data:image/jpeg;base64," + base64.b64encode(b"fake-jpeg").decode()

    result = await service.generate_assessment(
        profile={"age": 30},
        images=[image],
        posture_analysis={"has_analysis": True, "summaries": ["正面观轻微高低肩"]},
    )

    assert result["status"] == "completed"
    assert result["observations"][0]["kind"] == "posture_alignment"
    request = str(model.last_model_request_parameters)
    assert "正面观轻微高低肩" in request
