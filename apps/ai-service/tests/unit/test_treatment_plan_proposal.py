import pytest

from src.api.routes.treatment import TreatmentRecommendationRequest, recommend_treatment
from src.configuration.treatment_agent_config import get_default_treatment_configuration
from src.models.treatment import TreatmentAgentOutput
from src.services.treatment_plan_proposal import (
    PLAN_PROPOSAL_SCHEMA_REVISION,
    project_plan_proposal,
)

_DURABLE_ID_KEYS = {"templateId", "planExerciseId", "planId", "revisionId", "proposalId"}


def _emitted(**overrides) -> dict:
    payload = {
        "status": "proposed",
        "summary": "温和活动",
        "goal": "降低颈肩酸胀",
        "duration_weeks": 4,
        "interventions": [{"kind": "exercise", "title": "下巴微收", "description": "轻柔"}],
        "expected_timeline": "2 周",
        "evidence_ids": ["ev-1"],
        "governance": {"verdict": "accepted"},
        "plan": {
            "title": "颈肩计划",
            "sessions": [
                {
                    "title": "A 课",
                    "stages": [
                        {
                            "phase": "warmup",
                            "exercises": [
                                {
                                    "exercise_key": "neck_rotation",
                                    "name": "颈部转动",
                                    "sets": 1,
                                    "target": {"kind": "duration", "seconds": 30},
                                    "side": "both",
                                }
                            ],
                        },
                        {
                            "phase": "main",
                            "exercises": [
                                {
                                    "exercise_key": "chin_tuck",
                                    "name": "下巴微收",
                                    "sets": 2,
                                    "target": {"kind": "repetitions", "min": 8, "max": 12},
                                    "rest_seconds": 30,
                                    "instructions": "轻柔",
                                }
                            ],
                        },
                    ],
                }
            ],
            "sessions_per_week": 3,
            "frequency_description": "隔天一次",
            "constraints": ["疼痛加重时停止"],
        },
    }
    payload.update(overrides)
    return payload


def _keys(value) -> set[str]:
    if isinstance(value, dict):
        return set(value) | {k for item in value.values() for k in _keys(item)}
    if isinstance(value, list):
        return {k for item in value for k in _keys(item)}
    return set()


def test_projects_governed_plan_onto_strict_go_contract() -> None:
    validated = TreatmentAgentOutput.model_validate(_emitted()).model_dump(mode="json")
    validated["governance"] = {"verdict": "degraded"}
    block = project_plan_proposal(validated)

    assert block["schemaRevision"] == PLAN_PROPOSAL_SCHEMA_REVISION
    assert block["outcome"] == "proposed"
    assert block["title"] == "颈肩计划"
    assert block["goal"] == "降低颈肩酸胀"
    assert block["evidenceIds"] == ["ev-1"]
    content = block["content"]
    assert content["frequency"] == {"sessionsPerWeek": 3, "description": "隔天一次"}
    assert content["constraints"] == ["疼痛加重时停止"]
    warmup, main = content["sessions"][0]["stages"]
    assert warmup["exercises"][0] == {
        "exerciseId": "neck_rotation",
        "nameSnapshot": "颈部转动",
        "sets": 1,
        "target": {"kind": "duration", "seconds": 30.0},
        "side": "both",
    }
    assert main["exercises"][0] == {
        "exerciseId": "chin_tuck",
        "nameSnapshot": "下巴微收",
        "sets": 2,
        "target": {"kind": "repetitions", "min": 8.0, "max": 12.0},
        "restSeconds": 30.0,
        "instructions": "轻柔",
    }
    assert not _keys(block) & _DURABLE_ID_KEYS
    assert "schedule" not in content


def test_agent_supplied_ids_never_reach_the_block() -> None:
    raw = _emitted()
    raw["plan"]["sessions"][0]["templateId"] = "agent-template"
    raw["plan"]["sessions"][0]["stages"][1]["exercises"][0]["planExerciseId"] = "agent-entry"
    validated = TreatmentAgentOutput.model_validate(raw).model_dump(mode="json")
    validated["governance"] = {"verdict": "accepted"}

    assert not _keys(project_plan_proposal(validated)) & _DURABLE_ID_KEYS


@pytest.mark.parametrize(
    ("emitted", "reason"),
    [
        ({"governance": {"verdict": "rejected"}, "safety_fallback": "x"}, "governance_rejected"),
        (_emitted(governance={}), "governance_rejected"),
        (_emitted(plan=None), "no_plan"),
    ],
)
def test_rejected_or_planless_output_is_unavailable(emitted: dict, reason: str) -> None:
    assert project_plan_proposal(emitted) == {
        "schemaRevision": PLAN_PROPOSAL_SCHEMA_REVISION,
        "outcome": "unavailable",
        "reason": reason,
    }


def test_plan_is_optional_for_existing_treatment_outputs() -> None:
    assert TreatmentAgentOutput.model_validate(_emitted(plan=None)).plan is None


@pytest.mark.parametrize(
    "mutate",
    [
        lambda plan: plan.update(sessions=[]),
        lambda plan: plan["sessions"][0].update(stages=[]),
        lambda plan: plan["sessions"][0]["stages"][0].update(phase="stretch"),
        lambda plan: plan["sessions"][0]["stages"][0]["exercises"][0].update(sets=0),
        lambda plan: plan["sessions"][0]["stages"][0]["exercises"][0]["target"].update(
            kind="weight"
        ),
        lambda plan: plan.update(sessions_per_week=0),
    ],
)
def test_malformed_plan_draft_is_rejected_by_the_output_type(mutate) -> None:
    raw = _emitted()
    mutate(raw["plan"])
    with pytest.raises(ValueError):
        TreatmentAgentOutput.model_validate(raw)


@pytest.mark.asyncio
async def test_e2e_stub_route_emits_plan_proposal(monkeypatch) -> None:
    monkeypatch.setenv("BODYSENSE_E2E_STUB_AI", "1")
    monkeypatch.setenv("ENVIRONMENT", "test")
    request = TreatmentRecommendationRequest.model_validate(
        {
            "user_id": "user-1",
            "body_state_revision": 3,
            "configuration_id": get_default_treatment_configuration().configuration_id,
            "body_state": {"current_revision": 3},
            "diagnosis": {
                "currentRevision": 1,
                "conditionEntries": [
                    {
                        "conditionId": "condition-1",
                        "medicalName": "久坐相关颈肩负荷模式",
                        "disposition": "adopted",
                    }
                ],
            },
        }
    )

    result = await recommend_treatment(request)

    block = result["plan_proposal"]
    assert block["schemaRevision"] == PLAN_PROPOSAL_SCHEMA_REVISION
    assert block["outcome"] == "proposed"
    exercise = block["content"]["sessions"][0]["stages"][0]["exercises"][0]
    assert exercise["exerciseId"] == "chin_tuck"
    assert exercise["target"] == {"kind": "repetitions", "min": 8.0}
    assert not _keys(block) & _DURABLE_ID_KEYS
