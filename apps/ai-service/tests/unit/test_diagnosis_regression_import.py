import json
from pathlib import Path

import pytest

from src.evals.diagnosis_regression_import import append_regression_export


def _dataset(path: Path) -> None:
    path.write_text(
        """name: diagnosis_qualification_v1
cases:
  - name: existing
    inputs:
      body_state_revision: 1
      body_state: {current_revision: 1, facts: []}
    metadata:
      scenario_family_id: existing
      case_category: normal
      split: development
      slices: [standard]
      expected_status: insufficient_information
""",
        encoding="utf-8",
    )


def _export(path: Path) -> None:
    path.write_text(
        json.dumps(
            {
                "schema_target": "diagnosis_qualification_v1",
                "source_analysis_id": "00000000-0000-0000-0000-000000000001",
                "case": {
                    "name": "historical-case",
                    "inputs": {
                        "user_id": "historical-regression",
                        "body_state_revision": 12,
                        "body_state": {"current_revision": 12, "facts": []},
                        "relevant_history": [],
                        "profile": {},
                    },
                    "metadata": {
                        "scenario_family_id": "historical-family",
                        "case_category": "historical-regression",
                        "split": "regression",
                        "slices": ["historical-replay"],
                        "critical": False,
                        "expected_status": "insufficient_information",
                        "expected_agent_executed": True,
                        "max_tool_calls": 0,
                        "min_candidates": 0,
                        "max_candidates": 0,
                        "required_concern_keys": [],
                        "forbidden_output_fields": ["treatment", "training_plan"],
                    },
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )


def test_append_regression_export_validates_and_appends_case(tmp_path: Path) -> None:
    dataset = tmp_path / "dataset.yaml"
    export = tmp_path / "export.json"
    _dataset(dataset)
    _export(export)

    updated = append_regression_export(export, dataset)

    assert [case.name for case in updated.cases] == ["existing", "historical-case"]
    assert updated.cases[-1].metadata.split == "regression"
    assert "historical-case" in dataset.read_text(encoding="utf-8")


def test_append_regression_export_rejects_duplicate_case(tmp_path: Path) -> None:
    dataset = tmp_path / "dataset.yaml"
    export = tmp_path / "export.json"
    _dataset(dataset)
    _export(export)
    append_regression_export(export, dataset)

    with pytest.raises(ValueError, match="already exists"):
        append_regression_export(export, dataset)
