from src.evals.assessment_evidence_policy import (
    assessment_evidence_policy_summary,
    run_assessment_evidence_policy_qualification,
)


def test_assessment_evidence_contract_qualification_is_green() -> None:
    summary = assessment_evidence_policy_summary(run_assessment_evidence_policy_qualification())
    assert summary["total"] == 9
    assert summary["passed"] == summary["total"]
    assert summary["failed"] == 0
