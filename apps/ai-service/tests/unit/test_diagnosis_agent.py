"""Focused learning tests for the PydanticAI Diagnosis execution boundary."""

import pytest
from pydantic_ai.models.test import TestModel

from src.agents.diagnosis_agent import create_diagnosis_agent
from src.models.diagnosis import DiagnosisAgentOutput, DiagnosisDependencies


def _deps() -> DiagnosisDependencies:
    return DiagnosisDependencies(
        body_state_revision=12,
        body_state={
            "current_revision": 12,
            "facts": [
                {
                    "id": "fact-neck-1",
                    "kind": "discomfort",
                    "body_region": "颈肩",
                    "value": "久坐后酸胀",
                }
            ],
            "observations": [],
        },
        relevant_history=[{"revision": 11, "change_type": "fact.temporal_changed"}],
        profile={"age": 30},
    )


@pytest.mark.asyncio
async def test_agent_returns_typed_diagnosis_output_without_manual_json_parsing() -> None:
    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )
    agent = create_diagnosis_agent(model)

    result = await agent.run(
        "Synthesize possible diagnosis candidates from the supplied durable state.",
        deps=_deps(),
    )

    assert isinstance(result.output, DiagnosisAgentOutput)
    assert result.output.candidates[0].basis_fact_ids == ["fact-neck-1"]


@pytest.mark.asyncio
async def test_agent_run_context_receives_exact_body_state_revision() -> None:
    model = TestModel(
        custom_output_args={
            "status": "insufficient_information",
            "scope": "full_body",
            "summary": "现有信息不足。",
            "candidates": [],
            "cross_concern_patterns": [],
            "information_gaps": ["需要更多颈肩活动诱发信息"],
            "safety_summary": {},
        }
    )
    agent = create_diagnosis_agent(model)

    result = await agent.run("Analyze this run.", deps=_deps())

    assert result.output.status == "insufficient_information"
    request_parameters = model.last_model_request_parameters
    assert request_parameters is not None
    assert "R12" in str(request_parameters)
    assert "fact-neck-1" in str(request_parameters)
