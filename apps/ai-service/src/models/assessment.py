"""Typed evidence-grounded Assessment Agent contract.

The serving contract removes pseudo-precise health grades/scores from model
authority and makes evidence references mandatory on every generated
observation. Evidence coverage, gaps, report status and summary are derived by
application code, not authored by the model. Retired contracts live only in the
offline eval corpus and are not runtime-importable types.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field

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


@dataclass(slots=True)
class AssessmentDependencies:
    profile: dict[str, Any]
    body_state: dict[str, Any] = field(default_factory=dict)
    report_indicators: list[Any] = field(default_factory=list)
    reviewed_report_evidence: list[Any] = field(default_factory=list)
    posture_analysis: dict[str, Any] = field(default_factory=dict)
    rag_context: str = ""
    evidence_catalog: dict[str, dict[str, Any]] = field(default_factory=dict)
