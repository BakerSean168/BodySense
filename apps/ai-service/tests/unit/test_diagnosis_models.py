"""Target/domain tests for the BodyState-based Diagnosis models.

These tests intentionally encode the new ADR 0004 rules rather than preserving
old ``1..3`` candidate assumptions.
"""

import pytest
from pydantic import ValidationError

from src.models.diagnosis import (
    DiagnosisAgentOutput,
    DiagnosisCandidateDraft,
    DiagnosisConfidence,
    DiagnosisDependencies,
    DiagnosisSeverity,
)


def candidate(index: int = 1) -> DiagnosisCandidateDraft:
    return DiagnosisCandidateDraft(
        concern_key="region:neck",
        name=f"候选 {index}",
        confidence=DiagnosisConfidence.MEDIUM,
        severity=DiagnosisSeverity.MILD,
        basis="基于当前 BodyState 的姿态与不适信息",
        reasoning_summary="当前信息有一定匹配，但仍存在不确定性",
        basis_fact_ids=[f"fact-{index}"],
    )


def test_completed_analysis_requires_at_least_one_candidate() -> None:
    with pytest.raises(ValidationError):
        DiagnosisAgentOutput(status="completed", candidates=[])


def test_insufficient_information_can_return_zero_candidates() -> None:
    output = DiagnosisAgentOutput(
        status="insufficient_information",
        candidates=[],
        information_gaps=["缺少症状诱发场景"],
    )
    assert output.candidates == []


def test_candidate_count_is_not_capped_at_three() -> None:
    output = DiagnosisAgentOutput(
        status="completed",
        candidates=[candidate(index) for index in range(1, 9)],
    )
    assert len(output.candidates) == 8


def test_candidate_keeps_fact_observation_and_counterevidence_refs() -> None:
    item = DiagnosisCandidateDraft(
        name="头前伸姿态倾向",
        confidence=DiagnosisConfidence.HIGH,
        basis_fact_ids=["fact-a"],
        basis_observation_ids=["obs-a"],
        counterevidence_ids=["fact-negative-a"],
    )
    assert item.basis_fact_ids == ["fact-a"]
    assert item.basis_observation_ids == ["obs-a"]
    assert item.counterevidence_ids == ["fact-negative-a"]


def test_dependencies_pin_exact_body_state_revision() -> None:
    deps = DiagnosisDependencies(
        body_state_revision=42,
        body_state={"current_revision": 42, "facts": [], "observations": []},
        relevant_history=[{"revision": 41, "change_type": "fact.temporal_changed"}],
        profile={"age": 30},
    )
    assert deps.body_state_revision == 42
    assert deps.body_state["current_revision"] == 42
    assert deps.relevant_history[0]["revision"] == 41


def test_dependencies_slots_prevent_arbitrary_runtime_bag() -> None:
    deps = DiagnosisDependencies(body_state_revision=1, body_state={})
    with pytest.raises(AttributeError):
        deps.unplanned_field = "should fail"  # type: ignore[attr-defined]
