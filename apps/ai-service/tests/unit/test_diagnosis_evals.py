import copy
from pathlib import Path

from src.configuration.diagnosis_agent_config import CONFIG_ROOT, load_manifest
from src.evals.diagnosis_qualification import (
    DATASET_SCHEMA_PATH,
    DEFAULT_DATASET_PATH,
    compare_qualification_summaries,
    dataset_schema_json,
    load_diagnosis_dataset,
    report_summary,
    run_diagnosis_qualification,
)


def test_diagnosis_eval_dataset_is_versioned_typed_and_split_complete() -> None:
    dataset = load_diagnosis_dataset(DEFAULT_DATASET_PATH)
    assert dataset.name == "diagnosis_qualification_v1"
    assert len(dataset.cases) == 7
    assert {case.metadata.split for case in dataset.cases if case.metadata is not None} == {
        "development",
        "holdout",
        "regression",
        "challenge",
    }
    assert all(case.inputs.body_state_revision > 0 for case in dataset.cases)


def test_diagnosis_dataset_json_schema_is_repository_versioned() -> None:
    assert isinstance(DATASET_SCHEMA_PATH, Path)
    assert DATASET_SCHEMA_PATH.read_text(encoding="utf-8") == dataset_schema_json()


def test_diagnosis_qualification_is_slice_aware_and_critical_gate_green() -> None:
    run = run_diagnosis_qualification()
    summary = report_summary(run)
    assert summary["total"] == 7
    assert summary["failed"] == 0
    assert summary["configuration_id"].startswith("diag-config-")
    assert summary["configuration"]["role"] == "diagnosis"
    assert summary["qualification"]["qualified"] is True
    assert summary["slices"]["critical-safety"] == {"passed": 4, "total": 4}
    assert summary["splits"]["challenge"] == {"passed": 2, "total": 2}


def test_tool_trace_uses_current_evidence_tool_for_normal_cases() -> None:
    summary = report_summary(run_diagnosis_qualification())
    cases = {case["name"]: case for case in summary["cases"]}
    assert cases["current-severe-pain-blocks"]["trace"] == {
        "agent_executed": False,
        "available_tools": [],
        "tool_calls": [],
    }
    assert cases["mild-neck-load"]["trace"] == {
        "agent_executed": True,
        "available_tools": ["acquire_evidence"],
        "tool_calls": [],
    }


def test_paired_non_inferiority_blocks_critical_regression() -> None:
    champion = report_summary(run_diagnosis_qualification())
    same = compare_qualification_summaries(champion, champion)
    assert same["non_inferior"] is True
    assert same["promotion_eligible"] is True

    candidate = copy.deepcopy(champion)
    case = next(item for item in candidate["cases"] if item["name"] == "current-severe-pain-blocks")
    case["passed"] = False
    comparison = compare_qualification_summaries(champion, candidate)
    assert comparison["non_inferior"] is False
    assert comparison["critical_regressions"] == ["current-severe-pain-blocks"]


def test_qualification_dataset_path_is_repository_data_file() -> None:
    assert isinstance(DEFAULT_DATASET_PATH, Path)
    assert DEFAULT_DATASET_PATH.name == "diagnosis_qualification.yaml"
    assert DEFAULT_DATASET_PATH.exists()


def test_evidence_gap_challenger_is_paired_non_inferior_to_v1_champion() -> None:
    champion = report_summary(
        run_diagnosis_qualification(configuration_id="diag-config-f492eb1c0c6676ae")
    )
    challenger_config = load_manifest(CONFIG_ROOT / "diagnosis-v2-evidence-gap.yaml")
    challenger = report_summary(
        run_diagnosis_qualification(configuration_id=challenger_config.configuration_id)
    )

    comparison = compare_qualification_summaries(champion, challenger)

    assert challenger["qualification"]["qualified"] is True
    assert comparison["non_inferior"] is True
    assert comparison["promotion_eligible"] is True
    assert comparison["pass_rate_delta"] == 0.0
    assert comparison["critical_regressions"] == []
    cases = {case["name"]: case for case in challenger["cases"]}
    assert cases["mild-neck-load"]["trace"]["available_tools"] == ["acquire_evidence"]
