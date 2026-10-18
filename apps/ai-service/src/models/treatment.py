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


class TreatmentPlanTargetDraft(BaseModel):
    """One exercise target. Exactly the fields of its kind are expected."""

    kind: Literal["repetitions", "duration", "distance"]
    min: float | None = Field(default=None, gt=0, description="repetitions: minimum reps")
    max: float | None = Field(default=None, gt=0, description="repetitions: optional max reps")
    seconds: float | None = Field(default=None, gt=0, description="duration: seconds per set")
    meters: float | None = Field(default=None, gt=0, description="distance: meters per set")


class TreatmentPlanExerciseDraft(BaseModel):
    exercise_key: str = Field(
        min_length=1,
        max_length=120,
        description="Stable exercise concept key, e.g. chin_tuck. Never a durable ID.",
    )
    name: str = Field(min_length=1, max_length=200)
    sets: int = Field(ge=1, le=20)
    target: TreatmentPlanTargetDraft
    rest_seconds: float | None = Field(default=None, ge=0)
    side: Literal["left", "right", "both", "alternating"] | None = None
    load_or_intensity: str | None = None
    instructions: str | None = None


class TreatmentPlanStageDraft(BaseModel):
    phase: Literal["warmup", "main", "cooldown"]
    exercises: list[TreatmentPlanExerciseDraft] = Field(min_length=1)


class TreatmentPlanSessionDraft(BaseModel):
    title: str = Field(min_length=1, max_length=200)
    stages: list[TreatmentPlanStageDraft] = Field(min_length=1)


class TreatmentTrainingPlanDraft(BaseModel):
    """Session-based TrainingPlan draft. Go assigns every template/entry ID."""

    title: str = Field(min_length=1, max_length=200)
    sessions: list[TreatmentPlanSessionDraft] = Field(min_length=1, max_length=14)
    sessions_per_week: int | None = Field(default=None, ge=1, le=14)
    frequency_description: str | None = None
    constraints: list[str] = Field(default_factory=list)


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
    plan: TreatmentTrainingPlanDraft | None = Field(
        default=None,
        description=(
            "Session-based TrainingPlan draft for the user to review. Required for plan "
            "generation; it becomes a pending PlanProposal only, never an accepted plan."
        ),
    )

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
    diagnosis: dict[str, Any]
    profile: dict[str, Any] = field(default_factory=dict)
    user_constraints: dict[str, Any] = field(default_factory=dict)
    evidence: list[dict[str, Any]] = field(default_factory=list)
    evidence_searcher: EvidenceSearcher | None = None
    evidence_acquirer: EvidenceAcquirer | None = None
    retrieved_evidence: list[dict[str, Any]] = field(default_factory=list)
