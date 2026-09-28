from src.evals.diagnosis_negation_policy_v4 import (
    negation_policy_summary,
    run_negation_policy_qualification,
)


def test_v4_negation_list_policy_is_green_and_identity_bound() -> None:
    summary = negation_policy_summary(run_negation_policy_qualification())
    assert summary["passed"] == summary["total"] == 24
    assert summary["failed"] == 0
    assert summary["configuration_id"] == "diag-config-4eb948f419994367"
    assert summary["governance_policy_revision"] == "diagnosis-governance-v7-negation-list-claims"
    assert summary["detector_revision"] == "red-flag-detector-negation-list-v4"
