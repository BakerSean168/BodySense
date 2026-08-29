"""Typed contracts for the Consultation state-acquisition preflight."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

CONSULTATION_INTAKE_OUTPUT_SCHEMA_REVISION = "consultation-intake-output-v1"

ConsultationTurnKind = Literal[
    "symptom_report",
    "symptom_update",
    "correction",
    "general_question",
    "other",
]
LifestyleSection = Literal[
    "activity",
    "sleep",
    "exercise",
    "nutrition",
    "substances",
    "recovery",
]


class ConsultationSymptomDraft(BaseModel):
    """One explicit user-reported symptom candidate from the latest turn.

    The model may normalize wording, but must not infer a diagnosis, mechanism,
    or a field the user did not state. Go persists this as an unverified fact
    candidate until the user confirms it or answers a bound structured question.
    """

    body_part: str = Field(min_length=1, max_length=120)
    symptom_type: str = Field(min_length=1, max_length=200)
    duration: str = Field(default="", max_length=160)
    trigger: str = Field(default="", max_length=300)
    relief: str = Field(default="", max_length=300)
    severity: str = Field(default="", max_length=80)
    radiation: str = Field(default="", max_length=240)
    functional_impact: str = Field(default="", max_length=300)
    neurological_signs: str = Field(default="", max_length=300)
    onset: str = Field(default="", max_length=200)
    additional_notes: str = Field(default="", max_length=500)


class ConsultationLifestyleDraft(BaseModel):
    """One explicit current lifestyle statement from the latest turn."""

    section: LifestyleSection
    summary: str = Field(min_length=1, max_length=500)
    details: dict[str, Any] = Field(default_factory=dict)


class ConsultationIntakeOutput(BaseModel):
    """Typed latest-turn classification and explicit state candidates."""

    turn_kind: ConsultationTurnKind
    symptoms: list[ConsultationSymptomDraft] = Field(default_factory=list, max_length=3)
    lifestyle: list[ConsultationLifestyleDraft] = Field(default_factory=list, max_length=6)
    rationale: str = Field(default="", max_length=500)

    @model_validator(mode="after")
    def keep_general_questions_state_free(self) -> "ConsultationIntakeOutput":
        if self.turn_kind == "general_question" and (self.symptoms or self.lifestyle):
            raise ValueError("general_question must not create user state candidates")
        return self


def get_consultation_intake_output_type(
    revision: str = CONSULTATION_INTAKE_OUTPUT_SCHEMA_REVISION,
) -> type[ConsultationIntakeOutput]:
    if revision != CONSULTATION_INTAKE_OUTPUT_SCHEMA_REVISION:
        raise ValueError(f"unsupported Consultation intake output schema revision: {revision}")
    return ConsultationIntakeOutput


@dataclass(slots=True)
class ConsultationIntakeDependencies:
    latest_user_message: str
    profile: dict[str, Any] = field(default_factory=dict)
    body_state: dict[str, Any] = field(default_factory=dict)
