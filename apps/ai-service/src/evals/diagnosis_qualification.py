"""Pydantic Evals qualification system for immutable Diagnosis Agent configurations."""

from __future__ import annotations

from src.evals.diagnosis_qualification_dataset import (
    dataset_fingerprint,
    dataset_schema_json,
    load_dataset_document,
    load_diagnosis_dataset,
)
from src.evals.diagnosis_qualification_evaluators import (
    CandidatePolicy,
    ConfigurationProvenance,
    DiagnosisContract,
    ExpectedStatus,
    NoTreatmentSideEffect,
    ToolTracePolicy,
)
from src.evals.diagnosis_qualification_report import (
    compare_qualification_summaries,
    render_summary,
    report_summary,
    summary_json,
)
from src.evals.diagnosis_qualification_runtime import (
    build_deterministic_task,
    run_diagnosis_qualification,
)
from src.evals.diagnosis_qualification_schema import (
    DATASET_SCHEMA_PATH,
    DEFAULT_CHAMPION_REPORT_PATH,
    DEFAULT_DATASET_PATH,
    DEFAULT_QUALIFICATION_CONFIGURATION_ID,
    DEFAULT_QUALIFICATION_POLICY,
    SERVICE_ROOT,
    DiagnosisDatasetDocument,
    DiagnosisEvalCaseDocument,
    DiagnosisEvalExecution,
    DiagnosisEvalInputs,
    DiagnosisEvalMetadata,
    DiagnosisEvalTrace,
    DiagnosisQualificationRun,
    EvalContext,
    EvalSplit,
)

__all__ = [
    "DATASET_SCHEMA_PATH",
    "DEFAULT_CHAMPION_REPORT_PATH",
    "DEFAULT_DATASET_PATH",
    "DEFAULT_QUALIFICATION_CONFIGURATION_ID",
    "DEFAULT_QUALIFICATION_POLICY",
    "SERVICE_ROOT",
    "CandidatePolicy",
    "ConfigurationProvenance",
    "DiagnosisContract",
    "DiagnosisDatasetDocument",
    "DiagnosisEvalCaseDocument",
    "DiagnosisEvalExecution",
    "DiagnosisEvalInputs",
    "DiagnosisEvalMetadata",
    "DiagnosisEvalTrace",
    "DiagnosisQualificationRun",
    "EvalContext",
    "EvalSplit",
    "ExpectedStatus",
    "NoTreatmentSideEffect",
    "ToolTracePolicy",
    "build_deterministic_task",
    "compare_qualification_summaries",
    "dataset_fingerprint",
    "dataset_schema_json",
    "load_dataset_document",
    "load_diagnosis_dataset",
    "render_summary",
    "report_summary",
    "run_diagnosis_qualification",
    "summary_json",
]
