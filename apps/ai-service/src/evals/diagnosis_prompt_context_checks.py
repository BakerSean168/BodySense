"""Deterministic Diagnosis prompt-boundary evidence without provider/tokenizer guesses."""

from __future__ import annotations

import json
from copy import deepcopy
from typing import Any

from src.agents.diagnosis_agent import diagnosis_context_instructions
from src.agents.diagnosis_prompt_context import (
    SAFETY_DETAIL_KEYS,
    body_state_prompt_view,
    history_prompt_view,
    legacy_body_state_prompt_view,
    legacy_history_prompt_view,
)
from src.configuration.diagnosis_agent_config import SERVICE_ROOT
from src.models.diagnosis import DiagnosisDependencies
from src.models.safety import SafetyEnvelopeV2
from src.prompts.diagnosis import (
    DIAGNOSIS_CONTEXT_PROMPT_REVISION,
    DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION,
    DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION,
)


def representative_dependencies() -> DiagnosisDependencies:
    from src.evals.diagnosis_qualification import load_dataset_document

    document = load_dataset_document(
        SERVICE_ROOT / "data/evals/diagnosis_structured_safety_qualification.yaml"
    )
    envelope = document.cases[0].inputs.safety_envelope
    assert envelope is not None
    fact = {
        "id": "durable-fact-id",
        "source_ref": "body-state:fact:durable-fact-id",
        "value": "轻度酸胀",
        "review_state": "confirmed",
        "lifecycle_state": "active",
        "details": {
            **dict.fromkeys(SAFETY_DETAIL_KEYS, False),
            "safety_capture_revision": "body-state-safety-capture-v1",
            "duration": "三个月",
            "trigger": "久坐",
            "severity": "轻度",
            "improves_with_movement": True,
        },
    }
    history = [
        {
            "id": f"row-{i}",
            "user_id": "transport-user",
            "revision": i,
            "created_at": "2026-09-29T00:00:00Z",
            "change_type": "fact.updated",
            "source": "consultation",
            "changes": {
                "fact": deepcopy(fact),
                "note": f"REVISION-SENTINEL-{i}" + "久坐后出现酸胀，活动后缓解。" * 50,
            },
        }
        for i in range(3)
    ]
    return DiagnosisDependencies(
        body_state_revision=envelope.body_state_revision,
        body_state={
            "user_id": "transport-user",
            "current_revision": envelope.body_state_revision,
            "facts": [fact],
            "observations": [{"id": "observation-id", "value": "姿态观察"}],
            "hypotheses": [{"value": "负荷相关"}],
            "recent_revisions": deepcopy(history),
        },
        relevant_history=history,
        profile={"age": 35, "medical_history": ["既往记录"]},
        safety_envelope=SafetyEnvelopeV2.model_validate(envelope),
    )


def dynamic_views(instructions: str) -> dict[str, Any]:
    return {
        line.split(" JSON: ", 1)[0]: json.loads(line.split(" JSON: ", 1)[1])
        for line in instructions.splitlines()
        if " JSON: " in line
    }


def _pre_v9_reference(deps: DiagnosisDependencies, prompt_revision: str) -> str:
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


def _legacy_compatible_dependencies(deps: DiagnosisDependencies) -> DiagnosisDependencies:
    compatible = deepcopy(deps)
    compatible.body_state = legacy_body_state_prompt_view(deps.body_state)
    compatible.relevant_history = legacy_history_prompt_view(deps.relevant_history)
    return compatible


def _fact_boundary_checks() -> tuple[bool, bool]:
    fact = representative_dependencies().body_state["facts"][0]
    observation = {
        "id": "observation-boundary",
        "details": {
            "dizziness": "domain-value",
            "trauma": "domain-value",
            "weakness": "domain-value",
            "safety_capture_revision": "not-authority-here",
        },
    }
    body = {
        "facts": [deepcopy(fact)],
        "observations": [deepcopy(observation)],
        "hypotheses": [{"details": {"dizziness": "hypothesis-value"}}],
    }
    history = [
        {
            "id": "observation-row",
            "user_id": "transport-user",
            "revision": 9,
            "change_type": "observation.updated",
            "changes": {"before": deepcopy(observation), "after": deepcopy(observation)},
        },
        {
            "id": "context-row",
            "user_id": "transport-user",
            "revision": 10,
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
    body_view = body_state_prompt_view(body)
    legacy_body = legacy_body_state_prompt_view(body)
    history_view = history_prompt_view(history)
    legacy_history = legacy_history_prompt_view(history)
    non_fact_preserved = (
        body_view["observations"] == body["observations"]
        and body_view["hypotheses"] == body["hypotheses"]
        and legacy_body["observations"] == body["observations"]
        and history_view[0]["changes"] == history[0]["changes"]
        and legacy_history[0]["changes"] == history[0]["changes"]
        and history_view[1]["changes"]["observations"] == history[1]["changes"]["observations"]
        and legacy_history[1]["changes"]["observations"] == history[1]["changes"]["observations"]
    )
    fact_payloads = [
        history_view[1]["changes"]["facts"][0]["previous"],
        history_view[1]["changes"]["facts"][0]["replacement"],
        legacy_history[1]["changes"]["facts"][0]["previous"],
        legacy_history[1]["changes"]["facts"][0]["replacement"],
    ]
    composite_fact_filtered = all(
        not SAFETY_DETAIL_KEYS.intersection(payload["details"]) for payload in fact_payloads
    )
    return non_fact_preserved, composite_fact_filtered


def context_measurements() -> dict[str, float | int]:
    deps = representative_dependencies()
    old = diagnosis_context_instructions(deps, DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION)
    new = diagnosis_context_instructions(deps, DIAGNOSIS_CONTEXT_PROMPT_REVISION)
    old_size = sum(
        len(line.split(" JSON: ", 1)[1]) for line in old.splitlines() if " JSON: " in line
    )
    new_size = sum(
        len(line.split(" JSON: ", 1)[1]) for line in new.splitlines() if " JSON: " in line
    )
    return {"old_chars": old_size, "new_chars": new_size, "reduction": 1 - new_size / old_size}


def context_checks() -> dict[str, bool]:
    deps = representative_dependencies()
    frozen = deepcopy(deps)
    assert deps.safety_envelope is not None

    v9 = diagnosis_context_instructions(deps, DIAGNOSIS_CONTEXT_PROMPT_REVISION)
    v8 = diagnosis_context_instructions(deps, DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION)
    legacy = diagnosis_context_instructions(deps, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION)
    v9_views = dynamic_views(v9)
    legacy_views = dynamic_views(legacy)

    compatible = _legacy_compatible_dependencies(deps)
    historical = deepcopy(compatible)

    v9_fact_json = json.dumps([v9_views["BodyState"], v9_views["Relevant history"]])
    legacy_fact_json = json.dumps([legacy_views["BodyState"], legacy_views["Relevant history"]])
    non_fact_preserved, composite_fact_filtered = _fact_boundary_checks()

    return {
        "legacy_historical_prompt_byte_stable": diagnosis_context_instructions(
            historical, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION
        )
        == _pre_v9_reference(historical, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION),
        "v8_prompt_byte_stable": v8
        == _pre_v9_reference(deps, DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION),
        "fact_only_metadata_filter": all(
            f'"{key}"' not in v9_fact_json and f'"{key}"' not in legacy_fact_json
            for key in SAFETY_DETAIL_KEYS
        ),
        "non_fact_details_preserved": non_fact_preserved,
        "current_context_fact_payloads_filtered": composite_fact_filtered,
        "legacy_recent_revisions_duplication_preserved": "recent_revisions"
        in legacy_views["BodyState"]
        and all(legacy.count(f"REVISION-SENTINEL-{i}") == 2 for i in range(3)),
        "v9_recent_revisions_deduplicated": "recent_revisions" not in v9_views["BodyState"]
        and all(v9.count(f"REVISION-SENTINEL-{i}") == 1 for i in range(3)),
        "legacy_no_successor_fact_metadata": all(
            f'"{key}"' not in legacy_fact_json for key in SAFETY_DETAIL_KEYS
        ),
        "v9_no_successor_fact_metadata": all(
            f'"{key}"' not in v9_fact_json for key in SAFETY_DETAIL_KEYS
        ),
        "full_envelope_present": v9_views["SafetyEnvelopeV2"]
        == deps.safety_envelope.model_dump(mode="json"),
        "frozen_input_unchanged": deps == frozen,
        "context_reduction_at_least_25_percent": context_measurements()["reduction"] >= 0.25,
        "history_entries_and_fact_values_retained": len(v9_views["Relevant history"]) == 3
        and v9_fact_json.count("durable-fact-id") == 8,
        "profile_preserved": v9_views["Profile"] == deps.profile,
    }
