from pathlib import Path

from src.evals.diagnosis_baseline import (
    DEFAULT_DATASET_PATH,
    load_diagnosis_dataset,
    report_summary,
    run_diagnosis_baseline,
)


def test_diagnosis_eval_dataset_is_versioned_and_typed() -> None:
    dataset = load_diagnosis_dataset(DEFAULT_DATASET_PATH)
    assert dataset.name == "diagnosis_baseline"
    assert [case.name for case in dataset.cases] == [
        "mild-neck-load",
        "current-severe-pain-blocks",
        "historical-severe-pain-does-not-block",
    ]
    assert all(case.inputs.body_state_revision > 0 for case in dataset.cases)
    assert all(case.metadata is not None for case in dataset.cases)


def test_diagnosis_pydantic_evals_characterize_current_protected_behavior() -> None:
    report = run_diagnosis_baseline()
    summary = report_summary(report)
    assert summary["total"] == 3
    assert summary["failed"] == 0
    assert summary["slices"]["critical-safety"] == {"passed": 1, "total": 1}


def test_baseline_dataset_path_is_repository_data_file() -> None:
    assert isinstance(DEFAULT_DATASET_PATH, Path)
    assert DEFAULT_DATASET_PATH.name == "diagnosis_baseline.yaml"
    assert DEFAULT_DATASET_PATH.exists()
