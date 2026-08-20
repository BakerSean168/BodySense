"""Focused tests for typed Treatment proposal execution."""

import pytest
from pydantic_ai.models.test import TestModel

from src.agents.treatment_agent import create_treatment_agent
from src.models.treatment import TreatmentAgentOutput, TreatmentDependencies
from src.services.treatment_agent_service import TreatmentAgentService


def _proposal_output() -> dict:
    return {
        "status": "proposed",
        "summary": "以低风险颈肩负荷管理为主。",
        "goal": "降低久坐后的颈肩酸胀",
        "duration_weeks": 4,
        "interventions": [
            {
                "kind": "exercise",
                "title": "下巴微收",
                "description": "保持自然呼吸，轻柔完成。",
                "prescription": {
                    "sets": 2,
                    "reps": 8,
                    "frequency": "每日一次",
                    "stop_conditions": ["疼痛明显加重"],
                },
            }
        ],
        "daily_habits": ["每 45 分钟短暂活动"],
        "expected_timeline": "2 至 4 周观察趋势",
        "warning_signs": ["出现进行性无力时停止并寻求评估"],
        "review_triggers": ["症状持续加重"],
        "safety_notes": [],
        "evidence_ids": [],
    }


def _deps() -> TreatmentDependencies:
    return TreatmentDependencies(
        user_id="user-1",
        body_state_revision=18,
        body_state={"current_revision": 18, "facts": [{"id": "fact-neck"}]},
        diagnosis_analysis={"analysis_id": "analysis-1", "status": "completed"},
        candidate_assessments=[{"candidate_id": "candidate-1", "state": "confirmed"}],
    )


@pytest.mark.asyncio
async def test_treatment_agent_returns_typed_proposal() -> None:
    model = TestModel(custom_output_args=_proposal_output())
    agent = create_treatment_agent(model)

    result = await agent.run("Create a proposal.", deps=_deps())

    assert isinstance(result.output, TreatmentAgentOutput)
    assert result.output.status == "proposed"
    assert result.output.interventions[0].kind == "exercise"
    assert "R18" in str(model.last_model_request_parameters)


@pytest.mark.asyncio
async def test_treatment_service_keeps_proposal_shape_and_governance() -> None:
    model = TestModel(custom_output_args=_proposal_output())
    service = TreatmentAgentService(proposal_agent=create_treatment_agent(model))

    result = await service.recommend(
        user_id="user-1",
        body_state_revision=18,
        body_state=_deps().body_state,
        diagnosis_analysis=_deps().diagnosis_analysis,
        candidate_assessments=_deps().candidate_assessments,
        profile={},
        user_constraints={},
        evidence=[],
    )

    assert result["status"] == "proposed"
    assert result["governance"]["verdict"] == "accepted"
    assert result["interventions"][0]["title"] == "下巴微收"
