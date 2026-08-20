"""Import a reviewed historical Treatment export into the qualification dataset."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, ConfigDict, Field

from .treatment_qualification import (
    TreatmentDatasetDocument,
    TreatmentEvalCaseDocument,
)


class TreatmentRegressionExportEnvelope(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    schema_target: str = Field(min_length=1)
    source_revision_id: str = Field(min_length=1)
    case: TreatmentEvalCaseDocument


def load_treatment_regression_export(path: Path) -> TreatmentRegressionExportEnvelope:
    raw = json.loads(path.read_text(encoding="utf-8"))
    envelope = TreatmentRegressionExportEnvelope.model_validate(raw)
    if envelope.schema_target != "treatment_qualification_v1":
        raise ValueError(
            f"unsupported Treatment regression schema target: {envelope.schema_target}"
        )
    if envelope.case.metadata.split != "regression":
        raise ValueError("historical Treatment export must target the regression split")
    return envelope


def append_treatment_regression_export(
    export_path: Path,
    dataset_path: Path,
) -> TreatmentDatasetDocument:
    envelope = load_treatment_regression_export(export_path)
    raw: dict[str, Any] = yaml.safe_load(dataset_path.read_text(encoding="utf-8"))
    dataset = TreatmentDatasetDocument.model_validate(raw)
    if any(case.name == envelope.case.name for case in dataset.cases):
        raise ValueError(f"Treatment regression case already exists: {envelope.case.name}")

    updated = TreatmentDatasetDocument(
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
