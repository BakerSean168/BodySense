"""Versioned internal safety evidence projected by Go from a pinned BodyState."""

from datetime import datetime
from enum import StrEnum
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, JsonValue, StrictBool, StrictInt, model_validator


class SafetyConceptV1(StrEnum):
    TRAUMA = "trauma"
    RADIATING_PAIN = "radiating_pain"
    NUMBNESS = "numbness"
    WEAKNESS = "weakness"
    DIZZINESS = "dizziness"
    GAIT_INSTABILITY = "gait_instability"
    SEVERE_PAIN = "severe_pain"
    WORSENING = "worsening"
    INFECTION = "infection"
    SYSTEMIC = "systemic"
    UNKNOWN_RED_FLAG = "unknown_red_flag"


class SafetyPolarityV1(StrEnum):
    PRESENT = "present"
    ABSENT = "absent"
    UNCERTAIN = "uncertain"


class SafetyTemporalityV1(StrEnum):
    CURRENT = "current"
    HISTORICAL = "historical"
    RESOLVED = "resolved"


class SafetyReviewStateV1(StrEnum):
    CONFIRMED = "confirmed"
    UNVERIFIED = "unverified"


class SafetySourceKindV1(StrEnum):
    BODY_STATE_FACT = "body_state_fact"
    LEGACY_SAFETY_STATE = "legacy_safety_state"


class SafetyBlockerReasonV1(StrEnum):
    STRUCTURED_CURRENT_SIGNAL = "structured_current_signal"
    LEGACY_ACTIVE_STATE = "legacy_active_state"


class SafetyBlockerV1(BaseModel):
    model_config = ConfigDict(extra="forbid")

    concept: SafetyConceptV1
    source_ref: str = Field(min_length=1)
    source_kind: SafetySourceKindV1
    reason: SafetyBlockerReasonV1


class SafetyAssertionV1(BaseModel):
    model_config = ConfigDict(extra="forbid")

    concept: SafetyConceptV1
    polarity: SafetyPolarityV1
    temporality: SafetyTemporalityV1
    review_state: SafetyReviewStateV1
    source_ref: str = Field(min_length=1)
    source_kind: SafetySourceKindV1
    observed_at: datetime | None = None
    provenance: dict[str, JsonValue] | None = None


class SafetyCoverageV1(BaseModel):
    model_config = ConfigDict(extra="forbid")

    revision: Literal["body-state-safety-coverage-v1"]
    capture_revision: Literal["body-state-safety-capture-v1"]
    required_concepts: tuple[
        Literal["trauma"],
        Literal["radiating_pain"],
        Literal["numbness"],
        Literal["weakness"],
        Literal["dizziness"],
    ]
    covered_source_refs: list[str]
    incomplete_source_refs: list[str]
    complete: StrictBool

    @model_validator(mode="after")
    def validate_coverage(self) -> "SafetyCoverageV1":
        for refs in (self.covered_source_refs, self.incomplete_source_refs):
            if refs != sorted(set(refs)) or any(
                not ref.startswith("body-state:fact:") for ref in refs
            ):
                raise ValueError("coverage source refs must be unique sorted fact refs")
        if set(self.covered_source_refs) & set(self.incomplete_source_refs):
            raise ValueError("coverage source refs overlap")
        if self.complete != (bool(self.covered_source_refs) and not self.incomplete_source_refs):
            raise ValueError("coverage complete does not match source refs")
        return self


class SafetyEnvelopeV2(BaseModel):
    model_config = ConfigDict(extra="forbid")

    schema_revision: Literal["body-state-safety-envelope-v2"]
    policy_revision: Literal["body-state-safety-policy-v1"]
    body_state_revision: StrictInt = Field(gt=0)
    assertions: list[SafetyAssertionV1]
    active_blockers: list[SafetyBlockerV1]
    requires_review: StrictBool
    legacy_state_present: StrictBool
    coverage: SafetyCoverageV1

    @model_validator(mode="after")
    def validate_blockers(self) -> "SafetyEnvelopeV2":
        if self.requires_review != bool(self.active_blockers):
            raise ValueError("requires_review must match active_blockers")
        for blocker in self.active_blockers:
            expected_reason = (
                SafetyBlockerReasonV1.LEGACY_ACTIVE_STATE
                if blocker.source_kind == SafetySourceKindV1.LEGACY_SAFETY_STATE
                else SafetyBlockerReasonV1.STRUCTURED_CURRENT_SIGNAL
            )
            if blocker.reason != expected_reason:
                raise ValueError("blocker reason does not match source kind")
            if not any(
                assertion.concept == blocker.concept
                and assertion.source_ref == blocker.source_ref
                and assertion.source_kind == blocker.source_kind
                and assertion.temporality == SafetyTemporalityV1.CURRENT
                and assertion.polarity in (SafetyPolarityV1.PRESENT, SafetyPolarityV1.UNCERTAIN)
                for assertion in self.assertions
            ):
                raise ValueError("blocker has no matching current positive assertion")
        return self
