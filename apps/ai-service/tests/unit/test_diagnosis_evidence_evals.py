from src.evals.diagnosis_evidence_policy import (
    evidence_policy_summary,
    load_evidence_policy_dataset,
    run_evidence_policy_qualification,
)


def test_evidence_policy_dataset_covers_required_phase5_regressions() -> None:
    dataset = load_evidence_policy_dataset()
    assert [case.name for case in dataset.cases] == [
        "no-gap-no-search",
        "user-fact-never-rag",
        "external-gap-sufficient-result",
        "external-gap-contradictory-result",
        "external-gap-empty-published-corpus",
        "external-gap-search-unavailable",
        "external-gap-irrelevant-results",
        "critical-gap-zero-budget",
        "second-critical-gap-stops-at-budget",
    ]


def test_evidence_policy_pydantic_evals_are_green() -> None:
    summary = evidence_policy_summary(run_evidence_policy_qualification())
    assert summary["total"] == 9
    assert summary["failed"] == 0
