"""The internal safety envelope fixture is shared with the Go projector tests."""

import json
from copy import deepcopy
from pathlib import Path

import pytest
from pydantic import ValidationError

from src.api.routes.diagnosis import DiagnosisRequest
from src.models.safety import SafetyEnvelopeV2

FIXTURE = (
    Path(__file__).resolve().parents[4] / "contracts/internal/diagnosis/safety-envelope-v2.json"
)


@pytest.mark.parametrize(
    "case", json.loads(FIXTURE.read_text())["cases"], ids=lambda case: case["name"]
)
def test_shared_envelope_json_contract(case: dict) -> None:
    if case.get("error"):
        assert "envelope" not in case
        return
    envelope = SafetyEnvelopeV2.model_validate(case["envelope"])
    assert envelope.model_dump(mode="json", exclude_none=True) == case["envelope"]
    request = DiagnosisRequest.model_validate(
        {
            "body_state_revision": case["snapshot"]["current_revision"],
            "configuration_id": "historical-v3",
            "body_state": case["snapshot"],
            "safety_envelope": case["envelope"],
        }
    )
    assert request.safety_envelope == envelope


def test_unknown_safety_concept_and_inconsistent_blockers_rejected() -> None:
    clear = json.loads(FIXTURE.read_text())["cases"][0]["envelope"]
    with pytest.raises(ValidationError):
        SafetyEnvelopeV2.model_validate({**clear, "requires_review": True})
    with pytest.raises(ValidationError):
        SafetyEnvelopeV2.model_validate(
            {
                **clear,
                "assertions": [{**clear["assertions"][0], "concept": "new_unreviewed_concept"}],
            }
        )


@pytest.mark.parametrize(
    "change",
    [
        {"concept": "trauma"},
        {"source_ref": "body-state:fact:other"},
        {"source_kind": "legacy_safety_state"},
        {"reason": "legacy_active_state"},
    ],
)
def test_mismatched_typed_blocker_rejected(change: dict) -> None:
    blocked = deepcopy(json.loads(FIXTURE.read_text())["cases"][1]["envelope"])
    blocked["active_blockers"][0].update(change)
    with pytest.raises(ValidationError):
        SafetyEnvelopeV2.model_validate(blocked)


@pytest.mark.parametrize(
    "change",
    [
        {"polarity": "absent"},
        {"temporality": "resolved"},
    ],
)
def test_blocker_requires_current_positive_assertion(change: dict) -> None:
    blocked = deepcopy(json.loads(FIXTURE.read_text())["cases"][1]["envelope"])
    blocked["assertions"][1].update(change)
    with pytest.raises(ValidationError):
        SafetyEnvelopeV2.model_validate(blocked)


def test_optional_provenance_round_trip() -> None:
    envelope = json.loads(FIXTURE.read_text())["provenance_round_trip"]
    assert SafetyEnvelopeV2.model_validate(envelope).model_dump(
        mode="json", exclude_none=True
    ) == envelope


def test_internal_http_forwards_typed_envelope(client, monkeypatch) -> None:
    class FakeService:
        captured: dict | None = None

        async def generate_diagnosis(self, **kwargs):
            self.captured = kwargs
            return {"status": "insufficient_information", "candidates": []}

    fake = FakeService()
    monkeypatch.setattr("src.api.routes.diagnosis.get_diagnosis_service", lambda: fake)
    envelope = json.loads(FIXTURE.read_text())["cases"][0]["envelope"]
    response = client.post(
        "/api/diagnosis/analyze",
        json={
            "body_state_revision": 12,
            "configuration_id": "historical-v3",
            "body_state": {"current_revision": 12},
            "safety_envelope": envelope,
        },
    )
    assert response.status_code == 200
    assert fake.captured is not None
    assert isinstance(fake.captured["safety_envelope"], SafetyEnvelopeV2)
