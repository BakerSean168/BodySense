"""Typed Assessment selection/rendering and evidence-governance boundary tests."""

from __future__ import annotations

import base64

import pytest
from pydantic import ValidationError
from pydantic_ai.models.test import TestModel

from src.agents.assessment_agent import create_assessment_agent
from src.models.assessment import (
    ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
    AssessmentAgentOutput,
    AssessmentDependencies,
)
from src.prompts.assessment import ASSESSMENT_PROMPT_REVISION_V3
from src.services.assessment_service import AssessmentOutputRejectedError, AssessmentService


def _posture_output(evidence_ref: str = "posture:view:0:finding:0") -> dict:
    return {"observations": [{"kind": "posture_alignment", "evidence_refs": [evidence_ref]}]}


def _exercise_output() -> dict:
    return {"observations": [{"kind": "exercise_pattern", "evidence_refs": ["body_state:fact:0"]}]}


@pytest.mark.asyncio
async def test_v2_agent_can_only_select_kind_and_exact_evidence_ref() -> None:
    model = TestModel(custom_output_args=_posture_output())
    agent = create_assessment_agent(
        model,
        prompt_revision=ASSESSMENT_PROMPT_REVISION_V3,
        output_schema_revision=ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
    )
    result = await agent.run(
        "Select grounded observations.",
        deps=AssessmentDependencies(
            profile={"birth_date": "1996-08-27"},
            evidence_catalog={
                "posture:view:0:finding:0": {
                    "ref": "posture:view:0:finding:0",
                    "source": "posture_analysis",
                    "kind": "uneven_shoulders",
                    "value": {"label": "肩部对称性待复核"},
                }
            },
        ),
    )

    assert isinstance(result.output, AssessmentAgentOutput)
    assert result.output.model_dump(mode="json") == _posture_output()


def test_v2_schema_forbids_model_authored_observation_prose() -> None:
    with pytest.raises(ValidationError, match="Extra inputs are not permitted"):
        AssessmentAgentOutput.model_validate(
            {
                "observations": [
                    {
                        "kind": "exercise_pattern",
                        "evidence_refs": ["body_state:fact:0"],
                        "description": "建议增加运动频率",
                    }
                ]
            }
        )


@pytest.mark.asyncio
async def test_service_renders_posture_observation_from_trusted_posture_evidence() -> None:
    model = TestModel(custom_output_args=_posture_output())
    service = AssessmentService(model_resolver=lambda _config: model)
    result = await service.generate_assessment(
        profile={"birth_date": "1996-08-27"},
        body_state={"current_revision": 2, "facts": []},
        posture_analysis={
            "has_analysis": True,
            "views": [
                {
                    "view": "front",
                    "analysis": {
                        "findings": [
                            {
                                "key": "uneven_shoulders",
                                "label": "肩部对称性待复核",
                                "evidence": "右侧肩峰位置略高",
                            }
                        ]
                    },
                }
            ],
        },
    )

    observation = result["observations"][0]
    assert observation == {
        "kind": "posture_alignment",
        "body_region": "",
        "label": "肩部对称性待复核",
        "description": "体态分析记录：右侧肩峰位置略高。",
        "evidence_refs": ["posture:view:0:finding:0"],
    }
    assert result["evidence_coverage"]["domains"]["posture"]["status"] == "available"
    assert "health_grade" not in result
    assert "dimension_scores" not in result
    request = str(model.last_model_request_parameters)
    assert "posture:view:0:finding:0" in request
    # v3 receives the canonical evidence catalog, not duplicate full business context.
    assert "稳定用户档案" not in request
    assert "当前 BodyState" not in request


@pytest.mark.asyncio
async def test_service_renders_body_state_fact_without_claim_expansion() -> None:
    model = TestModel(custom_output_args=_exercise_output())
    service = AssessmentService(model_resolver=lambda _config: model)

    result = await service.generate_assessment(
        profile={"birth_date": "1996-08-27"},
        body_state={
            "current_revision": 1,
            "facts": [
                {"kind": "lifestyle.exercise", "value": "健身；频率：1-2"},
                {"kind": "lifestyle.sleep", "value": "规律"},
            ],
        },
    )

    observation = result["observations"][0]
    assert observation["label"] == "运动记录"
    assert observation["description"] == "来源记录：健身；频率：1-2。"
    assert "可能" not in observation["description"]
    assert "建议" not in observation["description"]
    domains = result["evidence_coverage"]["domains"]
    assert domains["exercise"]["status"] == "available"
    assert domains["lifestyle"]["status"] == "available"
    assert domains["posture"]["status"] == "missing"


@pytest.mark.asyncio
async def test_service_derives_insufficient_status_when_no_health_evidence_exists() -> None:
    from src.configuration.assessment_agent_config import get_default_assessment_configuration

    def fail_if_model_is_resolved(_config):
        raise AssertionError("no-evidence Assessment must not resolve or call a model")

    service = AssessmentService(model_resolver=fail_if_model_is_resolved)
    config = get_default_assessment_configuration()

    result = await service.generate_assessment(
        profile={"birth_date": "1996-08-27", "gender": "male"},
        body_state={"current_revision": 1, "facts": []},
        configuration_id=config.configuration_id,
    )

    assert result["status"] == "insufficient_information"
    assert result["evidence_coverage"]["status"] == "insufficient"
    assert result["evidence_coverage"]["available_sources"] == []
    assert len(result["evidence_gaps"]) == 6
    assert result["agent_configuration"]["id"] == config.configuration_id
    assert result["execution_provenance"]["status"] == "skipped_no_evidence"
    assert result["execution_provenance"]["usage"]["requests"] == 0


@pytest.mark.asyncio
async def test_service_rejects_posture_selection_without_posture_evidence() -> None:
    model = TestModel(custom_output_args=_posture_output())
    service = AssessmentService(model_resolver=lambda _config: model)

    with pytest.raises(AssessmentOutputRejectedError, match="evidence governance"):
        await service.generate_assessment(
            profile={"birth_date": "1996-08-27"},
            body_state={"facts": [{"kind": "lifestyle.sleep", "value": "轮班"}]},
            posture_analysis={},
        )


@pytest.mark.asyncio
async def test_service_does_not_reuse_unverified_body_state_as_evidence() -> None:
    def fail_if_model_is_resolved(_config):
        raise AssertionError("excluded evidence must not trigger model execution")

    service = AssessmentService(model_resolver=fail_if_model_is_resolved)
    result = await service.generate_assessment(
        profile={},
        body_state={
            "facts": [
                {
                    "kind": "lifestyle.exercise",
                    "value": "健身；频率：1-2",
                    "review_state": "unverified",
                    "excluded_from_reasoning": True,
                    "lifecycle_state": "active",
                }
            ]
        },
    )

    assert result["status"] == "insufficient_information"
    assert result["observations"] == []
    assert result["evidence_coverage"]["status"] == "insufficient"
    assert result["evidence_coverage"]["domains"]["exercise"]["status"] == "missing"
    assert result["execution_provenance"]["usage"]["requests"] == 0


@pytest.mark.asyncio
async def test_v2_rejects_raw_images_and_unmodeled_rag_context() -> None:
    model = TestModel(custom_output_args={"observations": []})
    service = AssessmentService(model_resolver=lambda _config: model)
    image = "data:image/jpeg;base64," + base64.b64encode(b"fake-jpeg").decode()

    with pytest.raises(ValueError, match="does not accept raw images"):
        await service.generate_assessment(profile={}, images=[image])

    with pytest.raises(ValueError, match="does not accept unmodeled rag_context"):
        await service.generate_assessment(profile={}, rag_context="hidden posture claim")
