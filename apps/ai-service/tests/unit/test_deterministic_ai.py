"""The explicit local/CI deterministic AI mode exercises real service seams."""

from __future__ import annotations

import pytest

from src.ai import AiRequest, AIService
from src.ai.types import ChatMessage
from src.configuration.diagnosis_agent_config import get_default_diagnosis_configuration
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
    assert "BodyState" in "".join(event.text or "" for event in events)
    assert events[-1].type == "done"
    assert any(event.type == "usage" for event in events)


@pytest.mark.asyncio
async def test_deterministic_typed_agents_keep_structured_contracts() -> None:
    diagnosis = DiagnosisService(model_resolver=lambda _config: deterministic_diagnosis_model())
    diagnosis_result = await diagnosis.generate_diagnosis(
        body_state_revision=1,
        configuration_id=get_default_diagnosis_configuration().configuration_id,
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

    assessment = AssessmentService(
        model_resolver=lambda _config: deterministic_assessment_model()
    )
    assessment_result = await assessment.generate_assessment(profile={"age": 30})
    assert assessment_result["observations"][0]["kind"] == "posture_alignment"
    assert "improvement_summary" not in assessment_result
