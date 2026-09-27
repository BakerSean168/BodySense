from src.evals.diagnosis_negation_policy import (
    load_negation_policy_dataset,
    negation_policy_summary,
    run_negation_policy_qualification,
)


def test_negation_policy_dataset_covers_review_boundaries() -> None:
    dataset = load_negation_policy_dataset()
    assert len(dataset.cases) == 10
    assert [case.name for case in dataset.cases] == [
        "legacy-v1-flags-negated-blocker",
        "v2-suppresses-exact-blocker",
        "negation-does-not-cross-source-boundary",
        "true-positive-trauma-radiating-dizziness",
        "mixed-trauma-negative-radiating-positive",
        "historical-negative-current-positive",
        "worsening-remains-positive",
        "ambiguous-not-denying-remains-positive",
        "ambiguous-not-confirming-remains-positive",
        "ambiguous-cannot-say-absence-remains-positive",
    ]


def test_negation_policy_is_green_and_identity_bound() -> None:
    summary = negation_policy_summary(run_negation_policy_qualification())
    assert summary["passed"] == summary["total"] == 10
    assert summary["failed"] == 0
    assert summary["configuration_id"] == "diag-config-375187050b203078"
    assert summary["governance_policy_revision"] == "diagnosis-governance-v5-negation-aware-claims"
    assert summary["detector_revision"] == "red-flag-detector-negation-aware-v2"
