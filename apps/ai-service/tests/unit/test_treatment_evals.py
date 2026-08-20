from src.configuration.treatment_agent_config import get_default_treatment_configuration
from src.evals.treatment_qualification import (
    load_dataset_document,
    report_summary,
    run_treatment_qualification,
)


def test_treatment_dataset_spans_required_baseline_splits() -> None:
    document = load_dataset_document()
    assert {case.metadata.split for case in document.cases} == {
        "development",
        "holdout",
        "regression",
        "challenge",
    }
    assert {case.name for case in document.cases} == {
        "confirmed-neck-candidate",
        "unsure-candidate-remains-reviewable",
        "user-constraint-is-in-run-context",
        "existing-evidence-is-pinned",
    }


def test_treatment_qualification_is_exact_config_and_all_green() -> None:
    run = run_treatment_qualification()
    summary = report_summary(run)
    config = get_default_treatment_configuration()

    assert summary["configuration_id"] == config.configuration_id
    assert summary["passed"] == 4
    assert summary["total"] == 4
    assert summary["qualification"]["qualified"] is True
    assert len(summary["dataset"]["fingerprint"]) == 64
