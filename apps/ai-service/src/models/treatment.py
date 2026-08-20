"""Typed Treatment proposal contracts for the longitudinal health loop."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

from .dependencies import EvidenceAcquirer, EvidenceSearcher

TREATMENT_OUTPUT_SCHEMA_REVISION = "treatment-output-v1"


class TreatmentInterventionOutput(BaseModel):
    kind: Literal["exercise", "mobility", "habit", "self_test", "education", "monitoring"]
    title: str = Field(min_length=1)
    description: str = Field(min_length=1)
    prescription: dict[str, Any] = Field(default_factory=dict)


class TreatmentAgentOutput(BaseModel):
    """AI proposal. It is not current until Go records explicit acceptance."""

    status: Literal["proposed"] = "proposed"
    summary: str = Field(min_length=1)
    goal: str = Field(min_length=1)
    duration_weeks: int = Field(ge=1, le=104)
    interventions: list[TreatmentInterventionOutput] = Field(min_length=1)
    daily_habits: list[str] = Field(default_factory=list)
    expected_timeline: str = Field(min_length=1)
    warning_signs: list[str] = Field(default_factory=list)
    review_triggers: list[str] = Field(default_factory=list)
    safety_notes: list[str] = Field(default_factory=list)
    evidence_ids: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def ensure_review_boundary(self) -> "TreatmentAgentOutput":
        if not self.review_triggers:
            self.review_triggers = ["症状明显加重或出现新的安全信号"]
        return self


def get_treatment_output_type(
    revision: str = TREATMENT_OUTPUT_SCHEMA_REVISION,
) -> type[TreatmentAgentOutput]:
    if revision != TREATMENT_OUTPUT_SCHEMA_REVISION:
        raise ValueError(f"unsupported Treatment output schema revision: {revision}")
    return TreatmentAgentOutput


@dataclass(slots=True)
class TreatmentDependencies:
    user_id: str
    body_state_revision: int
    body_state: dict[str, Any]
    diagnosis_analysis: dict[str, Any]
    candidate_assessments: list[dict[str, Any]] = field(default_factory=list)
    profile: dict[str, Any] = field(default_factory=dict)
    user_constraints: dict[str, Any] = field(default_factory=dict)
    evidence: list[dict[str, Any]] = field(default_factory=list)
    evidence_searcher: EvidenceSearcher | None = None
    evidence_acquirer: EvidenceAcquirer | None = None
    retrieved_evidence: list[dict[str, Any]] = field(default_factory=list)
