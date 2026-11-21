"""Load, fingerprint, and assemble the versioned Diagnosis qualification dataset."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

import yaml
from pydantic_evals import Case, Dataset

from src.evals.diagnosis_qualification_evaluators import (
    CandidatePolicy,
    ConfigurationProvenance,
    DiagnosisContract,
    ExpectedStatus,
    NoTreatmentSideEffect,
    ToolTracePolicy,
)
from src.evals.diagnosis_qualification_schema import (
    DEFAULT_DATASET_PATH,
    DiagnosisDatasetDocument,
    DiagnosisEvalExecution,
    DiagnosisEvalInputs,
    DiagnosisEvalMetadata,
)


def load_dataset_document(path: Path = DEFAULT_DATASET_PATH) -> DiagnosisDatasetDocument:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    return DiagnosisDatasetDocument.model_validate(raw)


def dataset_fingerprint(document: DiagnosisDatasetDocument) -> str:
    payload = document.model_dump(mode="json")
    for case in payload["cases"]:
        for optional in ("safety_envelope", "model_output"):
            if case["inputs"].get(optional) is None:
                case["inputs"].pop(optional, None)
        for optional in (
            "applicability_revision",
            "applicable_configuration_ids",
            "applicability_reason",
        ):
            if case["metadata"].get(optional) is None:
                case["metadata"].pop(optional, None)
    canonical = json.dumps(
        payload,
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(canonical.encode()).hexdigest()


def dataset_schema_json() -> str:
    return (
        json.dumps(
            DiagnosisDatasetDocument.model_json_schema(),
            ensure_ascii=False,
            indent=2,
            sort_keys=True,
        )
        + "\n"
    )


def load_diagnosis_dataset(
    path: Path = DEFAULT_DATASET_PATH,
    *,
    configuration_id: str | None = None,
) -> Dataset[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]:
    """Load typed YAML cases and attach deterministic BodySense evaluators."""

    document = load_dataset_document(path)
    # Structured-safety successors inherit the exact v8 case scope without rewriting
    # the frozen dataset. New immutable behavior revisions remain distinguishable by
    # configuration_id while applicability stays pinned to the same contract corpus.
    applicability_id = configuration_id
    if configuration_id is not None:
        applicability_id = {
            "diag-config-ba10b8e6820c3691": "diag-config-62d312942b76a154",
            "diag-config-3f64de162dc937ee": "diag-config-62d312942b76a154",
        }.get(configuration_id, configuration_id)
    cases = [
        Case(name=item.name, inputs=item.inputs, metadata=item.metadata)
        for item in document.cases
        if item.metadata.applicable_configuration_ids is None
        or configuration_id is None
        or configuration_id in item.metadata.applicable_configuration_ids
        or applicability_id in item.metadata.applicable_configuration_ids
    ]
    return Dataset(
        name=document.name,
        cases=cases,
        evaluators=[
            DiagnosisContract(),
            ExpectedStatus(),
            CandidatePolicy(),
            NoTreatmentSideEffect(),
            ConfigurationProvenance(),
            ToolTracePolicy(),
        ],
    )
