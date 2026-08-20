"""Import a reviewed historical Diagnosis export into the regression dataset."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, ConfigDict, Field

from .diagnosis_qualification import (
    DiagnosisDatasetDocument,
    DiagnosisEvalCaseDocument,
)


class DiagnosisRegressionExportEnvelope(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    schema_target: str = Field(min_length=1)
    source_analysis_id: str
    case: DiagnosisEvalCaseDocument


def load_regression_export(path: Path) -> DiagnosisRegressionExportEnvelope:
    raw = json.loads(path.read_text(encoding="utf-8"))
    envelope = DiagnosisRegressionExportEnvelope.model_validate(raw)
    if envelope.schema_target != "diagnosis_qualification_v1":
        raise ValueError(f"unsupported regression schema target: {envelope.schema_target}")
    if envelope.case.metadata.split != "regression":
        raise ValueError("historical export must target the regression split")
    return envelope


def append_regression_export(
    export_path: Path,
    dataset_path: Path,
) -> DiagnosisDatasetDocument:
    """Append one reviewed exported case while preserving dataset invariants."""

    envelope = load_regression_export(export_path)
    raw: dict[str, Any] = yaml.safe_load(dataset_path.read_text(encoding="utf-8"))
    dataset = DiagnosisDatasetDocument.model_validate(raw)
    if any(case.name == envelope.case.name for case in dataset.cases):
        raise ValueError(f"Diagnosis regression case already exists: {envelope.case.name}")

    updated = DiagnosisDatasetDocument(
        name=dataset.name,
        cases=[*dataset.cases, envelope.case],
    )
    dataset_path.write_text(
        yaml.safe_dump(
            updated.model_dump(mode="json", exclude_none=True),
            allow_unicode=True,
            sort_keys=False,
        ),
        encoding="utf-8",
    )
    return updated
