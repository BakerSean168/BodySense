"""Typed observation-only Assessment Agent contracts.

V1 is retained as an immutable historical contract for replay of the original
Assessment configurations. V2 removes pseudo-precise health grades/scores from
model authority and makes evidence references mandatory on every generated
observation. Evidence coverage, gaps, report status and summary are derived by
application code, not authored by the model.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

ASSESSMENT_OUTPUT_SCHEMA_REVISION = "assessment-output-v1"
ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2 = "assessment-output-v2"

AssessmentEvidenceSource = Literal[
    "body_state",
    "report",
    "posture_analysis",
]
AssessmentObservationKind = Literal[
    "posture_alignment",
    "posture_asymmetry",
    "lifestyle_pattern",
    "exercise_pattern",
    "report_indicator",
    "anthropometry",
]


# ---------------------------------------------------------------------------
# Historical v1 contract
# ---------------------------------------------------------------------------


class AssessmentDimensionScoresV1(BaseModel):
    posture: float = Field(ge=0, le=100)
    exercise: float = Field(ge=0, le=100)
    lifestyle: float = Field(ge=0, le=100)
    injury_risk: float = Field(ge=0, le=100)
    overall: float = Field(ge=0, le=100)


class AssessmentObservationDraftV1(BaseModel):
    kind: str = Field(min_length=1, max_length=80)
    body_region: str = Field(default="", max_length=120)
    label: str = Field(min_length=1, max_length=240)
    description: str = Field(min_length=1, max_length=2000)
    severity: Literal["轻度", "中度", "重度", "未知"] = "未知"
    confidence: Literal["高", "中", "低"] = "中"
    method: str = Field(default="assessment", max_length=80)
    condition: dict[str, Any] = Field(default_factory=dict)


class AssessmentAgentOutputV1(BaseModel):
    status: Literal["completed", "insufficient_information"]
    health_grade: Literal["A", "B", "C", "D"]
    dimension_scores: AssessmentDimensionScoresV1
    observations: list[AssessmentObservationDraftV1] = Field(default_factory=list)
    summary: str = Field(min_length=1, max_length=3000)
    information_gaps: list[str] = Field(default_factory=list)
    safety_notes: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def validate_completed_output(self) -> "AssessmentAgentOutputV1":
        if self.status == "completed" and not self.observations:
            raise ValueError("completed assessment requires at least one observation")
        return self


# ---------------------------------------------------------------------------
# Evidence-grounded v2 contract
# ---------------------------------------------------------------------------


class AssessmentObservationDraft(BaseModel):
    """One model-selected evidence classification, never durable prose.

    The model may classify exactly one existing evidence item into an
    observation ``kind``. It cannot author the label, description, body region,
    severity, confidence, or any recommendation text. Application code renders
    the durable observation deterministically from the trusted evidence item.
    """

    model_config = ConfigDict(extra="forbid")

    kind: AssessmentObservationKind
    evidence_refs: list[str] = Field(min_length=1, max_length=1)


class AssessmentAgentOutput(BaseModel):
    """V2 model authority: evidence selection/classification only."""

    model_config = ConfigDict(extra="forbid")

    observations: list[AssessmentObservationDraft] = Field(default_factory=list, max_length=24)


def get_assessment_output_type(
    revision: str = ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
) -> type[BaseModel]:
    if revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION:
        return AssessmentAgentOutputV1
    if revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2:
        return AssessmentAgentOutput
    raise ValueError(f"unsupported Assessment output schema revision: {revision}")


@dataclass(slots=True)
class AssessmentDependencies:
    profile: dict[str, Any]
    body_state: dict[str, Any] = field(default_factory=dict)
    report_indicators: list[Any] = field(default_factory=list)
    posture_analysis: dict[str, Any] = field(default_factory=dict)
    rag_context: str = ""
    evidence_catalog: dict[str, dict[str, Any]] = field(default_factory=dict)
