"""The v8 route consumes typed coverage and source-bound escalation evidence."""

import pytest
from pydantic_ai.models.test import TestModel

from src.configuration.diagnosis_agent_config import get_diagnosis_configuration
from src.models.safety import SafetyEnvelopeV2
from src.runtime.governance import guard_structured_output
from src.services.diagnosis_service import DiagnosisService

CONFIG_ID = "diag-config-62d312942b76a154"
REF = "body-state:fact:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
BODY = {
    "current_revision": 12,
    "facts": [
        {"id": REF.removeprefix("body-state:fact:"), "kind": "discomfort", "value": "轻度酸胀"}
    ],
    "observations": [],
}
OUTPUT = {
    "status": "completed",
    "scope": "full_body",
    "summary": "可能性分析",
    "candidates": [
        {
            "name": "负荷模式",
            "confidence": "中",
            "typical_symptoms": "头晕或麻木是典型症状教育，不是当前用户症状",
        }
    ],
    "safety_findings": [],
}


def test_v8_manifest_identity_is_computed_from_immutable_behavior():
    config = get_diagnosis_configuration(CONFIG_ID)
    assert config.configuration_id == CONFIG_ID
    assert config.decision_policy_revision == "diagnosis-decision-policy-v2-structured-safety"
    assert config.governance_policy_revision == "diagnosis-governance-v8-structured-safety"


def envelope(*, complete: bool = True, blocked: bool = False) -> SafetyEnvelopeV2:
    assertions = (
        [
            {
                "concept": "dizziness",
                "polarity": "present",
                "temporality": "current",
                "review_state": "confirmed",
                "source_ref": REF,
                "source_kind": "body_state_fact",
            }
        ]
        if blocked
        else []
    )
    blockers = (
        [
            {
                "concept": "dizziness",
                "source_ref": REF,
                "source_kind": "body_state_fact",
                "reason": "structured_current_signal",
            }
        ]
        if blocked
        else []
    )
    return SafetyEnvelopeV2.model_validate(
        {
            "schema_revision": "body-state-safety-envelope-v2",
            "policy_revision": "body-state-safety-policy-v1",
            "body_state_revision": 12,
            "assertions": assertions,
            "active_blockers": blockers,
            "requires_review": blocked,
            "legacy_state_present": False,
            "coverage": {
                "revision": "body-state-safety-coverage-v1",
                "capture_revision": "body-state-safety-capture-v1",
                "required_concepts": [
                    "trauma",
                    "radiating_pain",
                    "numbness",
                    "weakness",
                    "dizziness",
                ],
                "covered_source_refs": [REF] if complete else [],
                "incomplete_source_refs": [] if complete else [REF],
                "complete": complete,
            },
        }
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "complete,blocked,status",
    [(False, False, "insufficient_information"), (True, True, "safety_blocked")],
)
async def test_v8_preflight_bypasses_model(complete: bool, blocked: bool, status: str):
    model = TestModel(call_tools=[], custom_output_args=OUTPUT)
    result = await DiagnosisService(model_resolver=lambda _: model).generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=BODY,
        safety_envelope=envelope(complete=complete, blocked=blocked),
    )
    assert result["status"] == status
    assert result["candidates"] == []
    assert result["execution_provenance"]["status"] == "bypassed"
    assert model.last_model_request_parameters is None


@pytest.mark.asyncio
async def test_v8_education_does_not_trigger_prose_detector():
    model = TestModel(call_tools=[], custom_output_args=OUTPUT)
    result = await DiagnosisService(model_resolver=lambda _: model).generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=BODY,
        safety_envelope=envelope(),
    )
    assert result["status"] == "completed"
    assert result["governance"]["verdict"] == "accepted"


@pytest.mark.parametrize(
    "refs,verdict",
    [
        ([REF], "accepted"),
        (["body-state:fact:missing"], "rejected"),
        ([REF, REF], "rejected"),
        (["body-state:profile:abc"], "rejected"),
    ],
)
def test_v8_finding_source_resolution(refs: list[str], verdict: str):
    payload = {
        **OUTPUT,
        "safety_findings": [
            {
                "concept": "dizziness",
                "polarity": "uncertain",
                "temporality": "current",
                "source_refs": refs,
            }
        ],
    }
    result = guard_structured_output(
        "diagnosis",
        payload,
        policy_revision="diagnosis-governance-v8-structured-safety",
        body_state=BODY,
        safety_envelope=envelope(),
    )
    assert result.verdict == verdict
