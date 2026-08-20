import pytest
from pydantic import ValidationError

from src.api.routes.treatment import TreatmentRecommendationRequest
from src.configuration.treatment_agent_config import get_default_treatment_configuration


def _payload() -> dict:
    return {
        "user_id": "user-1",
        "body_state_revision": 12,
        "configuration_id": get_default_treatment_configuration().configuration_id,
        "body_state": {"current_revision": 12},
        "diagnosis_analysis": {"analysis_id": "analysis-1", "status": "completed"},
        "candidate_assessments": [{"candidate_id": "candidate-1", "state": "confirmed"}],
    }


def test_treatment_http_contract_requires_immutable_configuration_identity() -> None:
    payload = _payload()
    request = TreatmentRecommendationRequest.model_validate(payload)
    assert request.configuration_id.startswith("treat-config-")

    payload.pop("configuration_id")
    with pytest.raises(ValidationError):
        TreatmentRecommendationRequest.model_validate(payload)


def test_treatment_http_contract_rejects_legacy_model_routing_fields() -> None:
    payload = _payload()
    payload["use_case"] = "llm.json"
    with pytest.raises(ValidationError):
        TreatmentRecommendationRequest.model_validate(payload)
