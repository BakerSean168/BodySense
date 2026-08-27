"""Typed observation-only Assessment Agent contracts."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

ASSESSMENT_OUTPUT_SCHEMA_REVISION = "assessment-output-v1"


class AssessmentDimensionScores(BaseModel):
    posture: float = Field(ge=0, le=100)
    exercise: float = Field(ge=0, le=100)
    lifestyle: float = Field(ge=0, le=100)
    injury_risk: float = Field(ge=0, le=100)
    overall: float = Field(ge=0, le=100)


class AssessmentObservationDraft(BaseModel):
    """One reviewable observation candidate, never a diagnosis or prescription."""

    kind: str = Field(min_length=1, max_length=80)
    body_region: str = Field(default="", max_length=120)
    label: str = Field(min_length=1, max_length=240)
    description: str = Field(min_length=1, max_length=2000)
    severity: Literal["轻度", "中度", "重度", "未知"] = "未知"
    confidence: Literal["高", "中", "低"] = "中"
    method: str = Field(default="assessment", max_length=80)
    condition: dict[str, Any] = Field(default_factory=dict)


class AssessmentAgentOutput(BaseModel):
    status: Literal["completed", "insufficient_information"]
    health_grade: Literal["A", "B", "C", "D"]
    dimension_scores: AssessmentDimensionScores
    observations: list[AssessmentObservationDraft] = Field(default_factory=list)
    summary: str = Field(min_length=1, max_length=3000)
    information_gaps: list[str] = Field(default_factory=list)
    safety_notes: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def validate_completed_output(self) -> "AssessmentAgentOutput":
        if self.status == "completed" and not self.observations:
            raise ValueError("completed assessment requires at least one observation")
        return self


def get_assessment_output_type(
    revision: str = ASSESSMENT_OUTPUT_SCHEMA_REVISION,
) -> type[AssessmentAgentOutput]:
    if revision != ASSESSMENT_OUTPUT_SCHEMA_REVISION:
        raise ValueError(f"unsupported Assessment output schema revision: {revision}")
    return AssessmentAgentOutput


@dataclass(slots=True)
class AssessmentDependencies:
    profile: dict[str, Any]
    body_state: dict[str, Any] = field(default_factory=dict)
    report_indicators: list[Any] = field(default_factory=list)
    posture_analysis: dict[str, Any] = field(default_factory=dict)
    rag_context: str = ""
