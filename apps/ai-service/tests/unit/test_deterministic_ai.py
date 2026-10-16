"""The explicit local/CI deterministic AI mode exercises real service seams."""

from __future__ import annotations

import pytest

from src.ai import AiRequest, AIService
from src.ai.types import AiDoneEvent, AiTextDeltaEvent, AiUsageEvent, ChatMessage
from src.api.routes.assessment import AssessmentRequest, generate_assessment
from src.configuration.assessment_agent_config import get_default_assessment_configuration
from src.configuration.diagnosis_agent_config import get_diagnosis_configuration
from src.configuration.treatment_agent_config import get_default_treatment_configuration
from src.services.assessment_service import AssessmentService
from src.services.diagnosis_service import DiagnosisService
from src.services.treatment_agent_service import TreatmentAgentService
from src.testing_support.deterministic_ai import (
    deterministic_assessment_model,
    deterministic_diagnosis_model,
    deterministic_treatment_model,
)


@pytest.mark.asyncio
async def test_generic_ai_service_streams_without_provider_calls(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("BODYSENSE_DETERMINISTIC_AI", "true")
    service = AIService()
    events = [
        event
        async for event in service.generate_stream(
            AiRequest(
                use_case="consultation.reply",
                messages=[ChatMessage(role="user", content="hello")],
            )
        )
    ]
    assert "BodyState" in "".join(
        event.text for event in events if isinstance(event, AiTextDeltaEvent)
    )
    assert isinstance(events[-1], AiDoneEvent)
    assert any(isinstance(event, AiUsageEvent) for event in events)


@pytest.mark.asyncio
async def test_deterministic_typed_agents_keep_structured_contracts() -> None:
    diagnosis = DiagnosisService(model_resolver=lambda _config: deterministic_diagnosis_model())
    diagnosis_result = await diagnosis.generate_diagnosis(
        body_state_revision=1,
        configuration_id=get_diagnosis_configuration(
            "diag-config-5a4a13627e14b4cf"
        ).configuration_id,
        body_state={
            "current_revision": 1,
            "facts": [
                {
                    "id": "fact-1",
                    "kind": "discomfort",
                    "body_region": "颈肩",
                    "value": "久坐后酸胀",
                    "details": {"severity": "轻度"},
                }
            ],
            "observations": [],
        },
    )
    assert diagnosis_result["candidates"][0]["concern_key"] == "region:颈肩"
    assert diagnosis_result["governance"]["verdict"] == "accepted"

    treatment = TreatmentAgentService(
        model_resolver=lambda _config: deterministic_treatment_model()
    )
    treatment_result = await treatment.recommend(
        user_id="user-1",
        body_state_revision=1,
        configuration_id=get_default_treatment_configuration().configuration_id,
        body_state={"current_revision": 1},
        diagnosis_analysis=diagnosis_result,
        candidate_assessments=[{"candidate_id": "candidate-1", "state": "confirmed"}],
        profile={},
        user_constraints={},
        evidence=[],
    )
    assert treatment_result["status"] == "proposed"
    assert treatment_result["interventions"][0]["title"] == "下巴微收"

    assessment = AssessmentService(model_resolver=lambda _config: deterministic_assessment_model())
    assessment_result = await assessment.generate_assessment(
        profile={"gender": "female", "birth_date": "1996-08-27", "age_years": 30}
    )
    assert assessment_result["contract_revision"] == "assessment-output-v2"
    assert assessment_result["observations"] == []
    assert assessment_result["evidence_coverage"]["status"] == "insufficient"
    assert "health_grade" not in assessment_result
    assert "dimension_scores" not in assessment_result


@pytest.mark.asyncio
async def test_assessment_e2e_fixture_selects_only_asserted_native_activity(monkeypatch):
    monkeypatch.setenv("BODYSENSE_E2E_STUB_AI", "1")
    monkeypatch.setenv("ENVIRONMENT", "e2e")
    fact_id = "1a8fa06e-2041-4b4c-bf2c-d8f64af6b6ee"
    fact = {
        "id": fact_id,
        "kind": "lifestyle",
        "statement": "日常活动以久坐为主",
        "details": {"habit_type": "activity"},
        "assertion_status": "asserted",
    }
    result = await generate_assessment(
        AssessmentRequest(
            configuration_id=get_default_assessment_configuration().configuration_id,
            body_state={
                "facts": [
                    fact,
                    {
                        **fact,
                        "id": "68f609a9-11f8-4b01-b6ea-95fdfd694947",
                        "assertion_status": "withdrawn",
                    },
                ]
            },
        )
    )
    assert len(result["observations"]) == 1
    observation = result["observations"][0]
    assert observation["evidence_refs"] == [f"body_state:fact:{fact_id}"]
    assert observation["description"] == f"来源记录：{fact['statement']}。"
    assert result["governance"]["verdict"] == "accepted"
