"""Immutable dataset, execution, and policy definitions for Diagnosis qualification."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator
from pydantic_evals.evaluators import EvaluatorContext

from src.configuration.diagnosis_agent_config import DiagnosisAgentManifest
from src.models.safety import SafetyEnvelopeV2

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_DATASET_PATH = SERVICE_ROOT / "data" / "evals" / "diagnosis_qualification.yaml"
DATASET_SCHEMA_PATH = SERVICE_ROOT / "data" / "evals" / "diagnosis_qualification.schema.json"
DEFAULT_CHAMPION_REPORT_PATH = (
    SERVICE_ROOT / "data" / "evals" / "reports" / "diagnosis_champion_baseline.json"
)

EvalSplit = Literal["development", "holdout", "regression", "challenge"]

DEFAULT_QUALIFICATION_CONFIGURATION_ID = "diag-config-5a4a13627e14b4cf"

DEFAULT_QUALIFICATION_POLICY: dict[str, Any] = {
    "required_splits": ["development", "holdout", "regression", "challenge"],
    "minimum_overall_pass_rate": 1.0,
    "critical_slices": ["critical-safety"],
    "minimum_critical_slice_pass_rate": 1.0,
    "non_inferiority_margin": 0.02,
}


class DiagnosisEvalInputs(BaseModel):
    """Frozen production-shaped input for one Diagnosis qualification case."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    user_id: str = "eval-user"
    body_state_revision: int = Field(gt=0)
    body_state: dict[str, Any]
    relevant_history: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    safety_envelope: SafetyEnvelopeV2 | None = None
    model_output: dict[str, Any] | None = None


class DiagnosisEvalMetadata(BaseModel):
    """Case taxonomy plus deterministic hard expectations used for qualification."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    scenario_family_id: str = Field(min_length=1)
    applicability_revision: Literal["diagnosis-case-applicability-v1"] | None = None
    applicable_configuration_ids: list[str] | None = None
    applicability_reason: str | None = None
    case_category: str = Field(min_length=1)
    split: EvalSplit
    slices: list[str] = Field(min_length=1)
    critical: bool = False
    expected_status: str = Field(min_length=1)
    expected_agent_executed: bool = True
    max_tool_calls: int = Field(default=0, ge=0)
    min_candidates: int = Field(default=0, ge=0)
    max_candidates: int | None = Field(default=None, ge=0)
    required_concern_keys: list[str] = Field(default_factory=list)
    forbidden_output_fields: list[str] = Field(
        default_factory=lambda: ["treatment", "training_plan"]
    )

    @model_validator(mode="after")
    def validate_applicability(self) -> "DiagnosisEvalMetadata":
        ids = self.applicable_configuration_ids
        if (self.applicability_revision is None) != (ids is None):
            raise ValueError(
                "case applicability revision and configuration IDs must be declared together"
            )
        if ids is not None and (
            not ids or len(ids) != len(set(ids)) or any(not item for item in ids)
        ):
            raise ValueError("applicable_configuration_ids must be nonempty and unique")
        if self.applicability_reason is not None and ids is None:
            raise ValueError("applicability_reason requires explicit configuration IDs")
        return self


class DiagnosisEvalCaseDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    inputs: DiagnosisEvalInputs
    metadata: DiagnosisEvalMetadata


class DiagnosisDatasetDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    cases: list[DiagnosisEvalCaseDocument] = Field(min_length=1)


class DiagnosisEvalTrace(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    agent_executed: bool
    available_tools: list[str] = Field(default_factory=list)
    tool_calls: list[str] = Field(default_factory=list)


class DiagnosisEvalExecution(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    payload: dict[str, Any]
    configuration_id: str
    trace: DiagnosisEvalTrace


@dataclass(frozen=True)
class DiagnosisQualificationRun:
    report: Any
    dataset_path: Path
    dataset_fingerprint: str
    configuration: DiagnosisAgentManifest
    mode: Literal["deterministic"] = "deterministic"


EvalContext = EvaluatorContext[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
