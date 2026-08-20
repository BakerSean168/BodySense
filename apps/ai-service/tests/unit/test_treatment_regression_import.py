import json
from pathlib import Path

import pytest

from src.evals.treatment_regression_import import append_treatment_regression_export


def _dataset(path: Path) -> None:
    path.write_text(
        """name: treatment_qualification_v1
cases:
  - name: existing
    inputs:
      user_id: eval-existing
      body_state_revision: 1
      body_state: {current_revision: 1, facts: []}
      diagnosis_analysis: {analysis_id: analysis-existing, status: completed}
      candidate_assessments: [{candidate_id: candidate-existing, state: confirmed}]
    metadata:
      split: development
      slices: [standard]
      required_assessment_states: [confirmed]
      required_context_tokens: []
""",
        encoding="utf-8",
    )


def _export(path: Path) -> None:
    path.write_text(
        json.dumps(
            {
                "schema_target": "treatment_qualification_v1",
                "source_revision_id": "00000000-0000-0000-0000-000000000001",
                "case": {
                    "name": "historical-treatment",
                    "inputs": {
                        "user_id": "historical-regression",
                        "body_state_revision": 12,
                        "body_state": {"current_revision": 12, "facts": []},
                        "diagnosis_analysis": {
                            "analysis_id": "analysis-historical",
                            "status": "completed",
                        },
                        "candidate_assessments": [
                            {"candidate_id": "candidate-historical", "state": "confirmed"}
                        ],
                        "profile": {},
                        "user_constraints": {},
                        "evidence": [{"evidence_id": "evidence-historical"}],
                    },
                    "metadata": {
                        "split": "regression",
                        "slices": ["historical-replay"],
                        "required_assessment_states": ["confirmed"],
                        "required_context_tokens": ["evidence-historical"],
                    },
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )


def test_append_treatment_regression_export_validates_and_appends(tmp_path: Path) -> None:
    dataset = tmp_path / "dataset.yaml"
    export = tmp_path / "export.json"
    _dataset(dataset)
    _export(export)

    updated = append_treatment_regression_export(export, dataset)

    assert [case.name for case in updated.cases] == ["existing", "historical-treatment"]
    assert updated.cases[-1].metadata.split == "regression"
    assert "evidence-historical" in dataset.read_text(encoding="utf-8")


def test_append_treatment_regression_export_rejects_duplicate(tmp_path: Path) -> None:
    dataset = tmp_path / "dataset.yaml"
    export = tmp_path / "export.json"
    _dataset(dataset)
    _export(export)
    append_treatment_regression_export(export, dataset)

    with pytest.raises(ValueError, match="already exists"):
        append_treatment_regression_export(export, dataset)
