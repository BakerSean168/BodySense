"""Formal proposal projection: Agent judgment only, never ids/facts/adoption."""

from src.models.diagnosis import DiagnosisCandidateDraft
from src.services.diagnosis_formal_proposals import (
    FORMAL_PROPOSALS_SCHEMA_REVISION,
    build_formal_proposals,
)

FACT = "11111111-1111-4111-8111-111111111111"
OBS = "22222222-2222-4222-8222-222222222222"
BODY_STATE = {"current_revision": 3, "facts": [{"id": FACT}], "observations": [{"id": OBS}]}


def _candidate(**overrides):
    candidate = {
        "name": "Synthetic named syndrome",
        "confidence": "中",
        "basis": "Synthetic basis",
        "reasoning_summary": "Synthetic reasoning",
        "differential": "Synthetic differential",
        "basis_fact_ids": [FACT],
        "basis_observation_ids": [OBS],
        "counterevidence_ids": [],
        "missing_information": ["gap"],
        "assessment_level": "likely",
        "cause_summary": "Synthetic cause",
        "handling_summary": "Seek professional review",
        "handling_actions": ["record changes"],
        "caution_notes": ["stop if worse"],
    }
    candidate.update(overrides)
    return candidate


def _payload(candidates, status="completed", verdict="accepted"):
    return {
        "status": status,
        "summary": "s",
        "candidates": candidates,
        "governance": {"verdict": verdict},
    }


def test_projects_create_proposals_without_durable_identity():
    out = build_formal_proposals(_payload([_candidate()]), BODY_STATE)
    assert out["schemaRevision"] == FORMAL_PROPOSALS_SCHEMA_REVISION
    assert out["outcome"] == "proposals"
    (proposal,) = out["proposals"]
    assert proposal["changeKind"] == "create"
    assert proposal["medicalName"] == "Synthetic named syndrome"
    assert proposal["assessment"]["level"] == "likely"
    assert proposal["assessment"]["differentialNotes"] == ["Synthetic differential"]
    assert proposal["handlingPlan"] == {
        "summary": "Seek professional review",
        "actions": ["record changes"],
        "cautionNotes": ["stop if worse"],
    }
    assert [(e["evidenceType"], e["evidenceId"], e["role"]) for e in proposal["evidence"]] == [
        ("body_fact", FACT, "supporting"),
        ("body_observation", OBS, "supporting"),
    ]
    flat = repr(out)
    # Go assigns every identity, version, revision and owner; Agent never does.
    for forbidden in ("basedOnRunId", "evidenceVersion", "userId", "proposalId", "disposition"):
        assert forbidden not in flat


def test_counterevidence_is_typed_from_frozen_state_and_unknown_refs_are_dropped():
    out = build_formal_proposals(
        _payload(
            [_candidate(counterevidence_ids=[OBS, "kb-citation-7"], basis_fact_ids=[FACT, "x"])]
        ),
        BODY_STATE,
    )
    evidence = out["proposals"][0]["evidence"]
    assert [(e["evidenceId"], e["role"]) for e in evidence] == [
        (FACT, "supporting"),
        (OBS, "supporting"),
        (OBS, "counter"),
    ]
    assert evidence[2]["evidenceType"] == "body_observation"


def test_recoverable_native_gaps_are_repaired():
    out = build_formal_proposals(
        _payload([_candidate(assessment_level=None, cause_summary="", handling_summary=" ")]),
        BODY_STATE,
    )
    proposal = out["proposals"][0]
    assert proposal["assessment"]["level"] == "suspected"
    assert proposal["causeAnalysis"]["summary"]
    assert proposal["handlingPlan"]["summary"]
    high = build_formal_proposals(
        _payload([_candidate(assessment_level=None, confidence="高")]), BODY_STATE
    )
    assert high["proposals"][0]["assessment"]["level"] == "likely"


def test_candidates_without_resolvable_support_are_dropped():
    out = build_formal_proposals(
        _payload(
            [_candidate(basis_fact_ids=["unknown"], basis_observation_ids=[]), _candidate(name="B")]
        ),
        BODY_STATE,
    )
    assert [p["medicalName"] for p in out["proposals"]] == ["B"]
    none_left = build_formal_proposals(
        _payload([_candidate(basis_fact_ids=["unknown"], basis_observation_ids=[])]), BODY_STATE
    )
    assert none_left["outcome"] == "insufficient_evidence" and none_left["proposals"] == []


def test_non_proposal_outcomes_emit_zero_proposals():
    for payload in (
        _payload([], status="insufficient_information"),
        _payload([_candidate()], status="safety_blocked"),
        _payload([_candidate()], verdict="rejected"),
        _payload([]),
    ):
        out = build_formal_proposals(payload, BODY_STATE)
        assert out["outcome"] == "insufficient_evidence"
        assert out["proposals"] == []


def test_candidate_draft_native_fields_are_optional_and_bounded():
    legacy = DiagnosisCandidateDraft(name="x", confidence="中")
    assert legacy.assessment_level is None and legacy.handling_actions == []
    native = DiagnosisCandidateDraft(name="x", confidence="中", assessment_level="suspected")
    assert native.assessment_level == "suspected"
    try:
        DiagnosisCandidateDraft(name="x", confidence="中", assessment_level="confirmed")
    except ValueError:
        pass
    else:
        raise AssertionError("user-only level accepted from the Agent")
