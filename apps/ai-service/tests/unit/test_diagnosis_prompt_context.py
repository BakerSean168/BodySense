"""Characterize immutable Diagnosis prompt context and the v9 successor boundary."""

from __future__ import annotations

import json
from copy import deepcopy

import pytest
from pydantic_ai.models.test import TestModel

from src.agents.diagnosis_agent import create_diagnosis_agent, diagnosis_context_instructions
from src.agents.diagnosis_prompt_context import (
    SAFETY_DETAIL_KEYS,
    body_state_prompt_view,
    history_prompt_view,
    legacy_body_state_prompt_view,
    legacy_history_prompt_view,
    profile_prompt_view,
)
from src.configuration.diagnosis_agent_config import CONFIG_ROOT, SERVICE_ROOT, load_manifest
from src.evals.diagnosis_promotion import evaluate_promotion_readiness, load_promotion_policy
from src.evals.diagnosis_prompt_context_checks import (
    context_checks,
    dynamic_views,
    representative_dependencies,
)
from src.evals.diagnosis_qualification import load_diagnosis_dataset
from src.prompts.diagnosis import (
    DIAGNOSIS_CONTEXT_PROMPT_REVISION,
    DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION,
    DIAGNOSIS_PROMPT_REVISION,
    DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION,
)


def _pre_v9_reference(deps, prompt_revision: str) -> str:
    instructions = (
        "Analyze the exact durable BodyState revision supplied for this run.\n"
        f"BodyState revision: R{deps.body_state_revision}\n"
        f"BodyState JSON: {json.dumps(deps.body_state, ensure_ascii=False)}\n"
        f"Relevant history JSON: {json.dumps(deps.relevant_history, ensure_ascii=False)}\n"
        f"Profile JSON: {json.dumps(deps.profile, ensure_ascii=False)}\n"
        "Use acquire_evidence only for a typed, material EvidenceGap. "
        "User facts must use kind=user_fact and can never be supplied by RAG."
    )
    if prompt_revision == DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION:
        assert deps.safety_envelope is not None
        instructions += "\nSafetyEnvelopeV2 JSON: " + deps.safety_envelope.model_dump_json()
    return instructions


def _legacy_compatible_reference(deps):
    compatible = deepcopy(deps)
    compatible.body_state = legacy_body_state_prompt_view(deps.body_state)
    compatible.relevant_history = legacy_history_prompt_view(deps.relevant_history)
    return compatible


def _historical_deps_without_successor_metadata():
    return _legacy_compatible_reference(representative_dependencies())


@pytest.mark.parametrize(
    "prompt_revision", [DIAGNOSIS_PROMPT_REVISION, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION]
)
def test_legacy_historical_prompt_is_byte_stable(prompt_revision):
    deps = _historical_deps_without_successor_metadata()
    assert diagnosis_context_instructions(deps, prompt_revision) == _pre_v9_reference(
        deps, prompt_revision
    )


@pytest.mark.parametrize("version", [3, 4, 5, 6, 7])
def test_v3_to_v7_manifests_keep_legacy_serialization(version):
    config = load_manifest(next(CONFIG_ROOT.glob(f"diagnosis-v{version}-*.yaml")))
    deps = _historical_deps_without_successor_metadata()
    assert config.prompt_revision == DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION
    assert diagnosis_context_instructions(deps, config.prompt_revision) == _pre_v9_reference(
        deps, config.prompt_revision
    )


def test_legacy_current_input_only_filters_successor_fact_metadata():
    deps = representative_dependencies()
    frozen = deepcopy(deps)
    expected_deps = _legacy_compatible_reference(deps)
    actual = diagnosis_context_instructions(deps, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION)
    expected = _pre_v9_reference(expected_deps, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION)
    assert actual == expected

    views = dynamic_views(actual)
    assert "recent_revisions" in views["BodyState"]
    assert "user_id" in views["BodyState"]
    for i in range(3):
        assert actual.count(f"REVISION-SENTINEL-{i}") == 2
    assert '"details": {' in actual
    serialized = json.dumps([views["BodyState"], views["Relevant history"]])
    assert all(f'"{key}"' not in serialized for key in SAFETY_DETAIL_KEYS)
    assert views["BodyState"]["facts"][0]["details"]["duration"] == "三个月"
    assert deps == frozen


def test_v8_historical_prompt_construction_remains_exact():
    deps = representative_dependencies()
    actual = diagnosis_context_instructions(deps, DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION)
    assert actual == _pre_v9_reference(deps, DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION)
    assert '"safety_capture_revision"' in actual
    assert actual.count("REVISION-SENTINEL-0") == 2


def test_v9_deduplicates_context_and_preserves_full_envelope():
    deps = representative_dependencies()
    frozen = deepcopy(deps)
    instructions = diagnosis_context_instructions(deps, DIAGNOSIS_CONTEXT_PROMPT_REVISION)
    views = dynamic_views(instructions)
    body, history = views["BodyState"], views["Relevant history"]

    assert "recent_revisions" not in body
    assert "user_id" not in body
    assert len(history) == 3
    for i, row in enumerate(history):
        assert instructions.count(f"REVISION-SENTINEL-{i}") == 1
        assert "id" not in row and "user_id" not in row
        fact = row["changes"]["fact"]
        assert fact["id"] == "durable-fact-id"
        assert fact["source_ref"] == "body-state:fact:durable-fact-id"
        assert fact["details"] == {
            "duration": "三个月",
            "trigger": "久坐",
            "severity": "轻度",
            "improves_with_movement": True,
        }
    assert views["SafetyEnvelopeV2"] == deps.safety_envelope.model_dump(mode="json")
    assert deps == frozen


def test_non_fact_details_are_never_treated_as_safety_authority():
    body = {
        "facts": [
            {
                "id": "f",
                "value": "dizziness",
                "details": {"dizziness": False, "duration": "一周"},
            }
        ],
        "observations": [
            {
                "id": "o",
                "details": {
                    "dizziness": "observed-domain-value",
                    "trauma": "observation-domain-value",
                    "weakness": "scale-label",
                    "safety_capture_revision": "not-authority-here",
                },
            }
        ],
        "hypotheses": [{"details": {"dizziness": "hypothesis-domain-value"}}],
        "current_context": {"details": {"trauma": "context-domain-value"}},
    }
    v9 = body_state_prompt_view(body)
    legacy = legacy_body_state_prompt_view(body)
    assert v9["facts"][0]["details"] == {"duration": "一周"}
    assert legacy["facts"][0]["details"] == {"duration": "一周"}
    assert v9["observations"] == body["observations"]
    assert legacy["observations"] == body["observations"]
    assert v9["hypotheses"] == body["hypotheses"]
    assert legacy["current_context"] == body["current_context"]


def test_history_sanitizes_only_fact_revision_payloads():
    fact = representative_dependencies().body_state["facts"][0]
    observation = {
        "id": "obs",
        "details": {
            "dizziness": "observation-domain-value",
            "safety_capture_revision": "not-authority-here",
        },
    }
    rows = [
        {
            "id": "r1",
            "user_id": "u",
            "revision": 1,
            "change_type": "fact.updated",
            "changes": {"before": deepcopy(fact), "after": deepcopy(fact)},
        },
        {
            "id": "r2",
            "user_id": "u",
            "revision": 2,
            "change_type": "fact.corrected",
            "changes": {"previous": deepcopy(fact), "replacement": deepcopy(fact)},
        },
        {
            "id": "r3",
            "user_id": "u",
            "revision": 3,
            "change_type": "fact.added",
            "changes": {"fact": deepcopy(fact)},
        },
        {
            "id": "r4",
            "user_id": "u",
            "revision": 4,
            "change_type": "fact.candidate_accepted",
            "changes": {"candidate": deepcopy(fact), "previous": deepcopy(fact)},
        },
        {
            "id": "r5",
            "user_id": "u",
            "revision": 5,
            "change_type": "observation.updated",
            "changes": {"before": deepcopy(observation), "after": deepcopy(observation)},
        },
        {
            "id": "r6",
            "user_id": "u",
            "revision": 6,
            "change_type": "current_context.updated",
            "changes": {
                "facts": [
                    {
                        "kind": "activity",
                        "action": "transitioned",
                        "previous": deepcopy(fact),
                        "replacement": deepcopy(fact),
                    }
                ],
                "observations": [
                    {
                        "kind": "posture",
                        "action": "transitioned",
                        "previous": deepcopy(observation),
                        "replacement": deepcopy(observation),
                    }
                ],
            },
        },
    ]
    frozen = deepcopy(rows)
    legacy = legacy_history_prompt_view(rows)
    v9 = history_prompt_view(rows)

    for collection in (legacy, v9):
        for row in collection[:4]:
            for key, payload in row["changes"].items():
                if key in {"fact", "before", "after", "previous", "replacement", "candidate"}:
                    assert not SAFETY_DETAIL_KEYS.intersection(payload["details"])
                    assert payload["details"]["duration"] == "三个月"
        assert collection[4]["changes"] == rows[4]["changes"]
        composite_fact = collection[5]["changes"]["facts"][0]
        assert not SAFETY_DETAIL_KEYS.intersection(composite_fact["previous"]["details"])
        assert not SAFETY_DETAIL_KEYS.intersection(composite_fact["replacement"]["details"])
        assert collection[5]["changes"]["observations"] == rows[5]["changes"]["observations"]
    assert legacy[0]["id"] == "r1" and legacy[0]["user_id"] == "u"
    assert "id" not in v9[0] and "user_id" not in v9[0]
    assert rows == frozen


def test_views_are_deep_copies_and_preserve_profile():
    deps = representative_dependencies()
    frozen = deepcopy(deps)
    body = body_state_prompt_view(deps.body_state)
    history = history_prompt_view(deps.relevant_history)
    profile = profile_prompt_view(deps.profile)
    body["facts"][0]["value"] = "changed"
    history[0]["changes"]["fact"]["details"]["duration"] = "changed"
    profile["medical_history"].append("changed")
    assert deps == frozen


def test_unknown_context_revision_fails_closed():
    with pytest.raises(ValueError, match="unsupported Diagnosis prompt revision"):
        diagnosis_context_instructions(representative_dependencies(), "diagnosis-prompt-v999")


def test_all_policy_context_invariants():
    checks = context_checks()
    assert all(checks.values()), checks


def test_v9_exact_identity_and_only_prompt_differs():
    v8 = load_manifest(CONFIG_ROOT / "diagnosis-v8-structured-safety.yaml")
    v9 = load_manifest(CONFIG_ROOT / "diagnosis-v9-structured-safety-context.yaml")
    assert v8.configuration_id == "diag-config-62d312942b76a154"
    assert v9.configuration_id == "diag-config-ba10b8e6820c3691"
    assert v9.prompt_revision == DIAGNOSIS_CONTEXT_PROMPT_REVISION
    assert v8.model_dump(exclude={"prompt_revision"}) == v9.model_dump(exclude={"prompt_revision"})
    dataset = SERVICE_ROOT / "data/evals/diagnosis_structured_safety_qualification.yaml"
    assert [
        c.name for c in load_diagnosis_dataset(dataset, configuration_id=v8.configuration_id).cases
    ] == [
        c.name for c in load_diagnosis_dataset(dataset, configuration_id=v9.configuration_id).cases
    ]


def test_v9_promotion_lineage_and_readiness():
    policy = load_promotion_policy(SERVICE_ROOT / "data/evals/diagnosis_promotion_policy_v7.json")
    report = evaluate_promotion_readiness(policy)
    assert report["ready_for_shadow"] is True, report
    assert policy.champion_configuration_id == "diag-config-5a4a13627e14b4cf"
    assert policy.challenger_configuration_id == "diag-config-ba10b8e6820c3691"
    raw = json.loads((SERVICE_ROOT / "data/evals/diagnosis_promotion_policy_v7.json").read_text())
    assert raw["interaction_experiment"]["required"] is True
    assert [r["configuration_id"] for r in raw["qualification_chain"]] == [
        "diag-config-5a4a13627e14b4cf",
        "diag-config-62d312942b76a154",
        "diag-config-ba10b8e6820c3691",
    ]


async def test_agent_actually_uses_v9_context_view():
    deps = representative_dependencies()
    agent = create_diagnosis_agent(
        TestModel(
            call_tools=[],
            custom_output_args={"status": "insufficient_information", "summary": "信息不足"},
        ),
        prompt_revision=DIAGNOSIS_CONTEXT_PROMPT_REVISION,
        output_schema_revision="diagnosis-output-v3-structured-safety",
    )
    result = await agent.run("Analyze", deps=deps)
    instructions = next(
        message.instructions
        for message in result.all_messages()
        if getattr(message, "instructions", None)
    )
    assert instructions.count("REVISION-SENTINEL-0") == 1
    assert dynamic_views(instructions)["SafetyEnvelopeV2"] == deps.safety_envelope.model_dump(
        mode="json"
    )


async def test_v9_governance_receives_full_frozen_authority(monkeypatch):
    import src.services.diagnosis_service as service

    deps = representative_dependencies()
    frozen = deepcopy(deps)
    seen = []
    original = service.guard_structured_output

    def capture(*args, **kwargs):
        seen.append((kwargs["body_state"], kwargs["safety_envelope"]))
        return original(*args, **kwargs)

    monkeypatch.setattr(service, "guard_structured_output", capture)
    model = TestModel(
        call_tools=[],
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "可能性分析",
            "candidates": [{"name": "负荷模式", "confidence": "中"}],
            "safety_findings": [],
        },
    )
    result = await service.DiagnosisService(model_resolver=lambda _: model).generate_diagnosis(
        body_state_revision=deps.body_state_revision,
        configuration_id="diag-config-ba10b8e6820c3691",
        body_state=deps.body_state,
        relevant_history=deps.relevant_history,
        profile=deps.profile,
        safety_envelope=deps.safety_envelope,
    )
    assert result["status"] == "completed"
    assert seen and seen[-1][0] is deps.body_state and seen[-1][1] is deps.safety_envelope
    assert deps == frozen
