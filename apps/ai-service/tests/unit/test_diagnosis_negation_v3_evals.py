from src.evals.diagnosis_negation_policy_v3 import (
    negation_policy_summary,
    run_negation_policy_qualification,
)


def test_v3_negation_policy_is_green_and_identity_bound() -> None:
    summary = negation_policy_summary(run_negation_policy_qualification())
    assert summary["passed"] == summary["total"] == 14
    assert summary["failed"] == 0
    assert summary["configuration_id"] == "diag-config-4377355ba2012ce8"
    assert summary["governance_policy_revision"] == "diagnosis-governance-v6-negation-bridge-claims"
    assert summary["detector_revision"] == "red-flag-detector-negation-bridge-v3"
