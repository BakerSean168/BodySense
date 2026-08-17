"""Typed internal models for the BodyState-based Diagnosis reasoning boundary.

ADR 0004 changes the core input/output semantics:

    exact BodyState revision + bounded temporal history
        -> Diagnosis reasoning
        -> zero-to-many candidate drafts
        -> Go assigns durable analysis/candidate identities

The Python model owns reasoning structure, not durable business identity. This is
why candidate_id / analysis_id are intentionally absent from these models.
"""

from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

from .dependencies import EvidenceSearcher


class DiagnosisConfidence(StrEnum):
    """How well a candidate fits the available BodyState evidence."""

    HIGH = "高"
    MEDIUM = "中"
    LOW = "低"


class DiagnosisSeverity(StrEnum):
    """Current impact/severity when the available evidence supports expressing it.

    Severity is deliberately separate from confidence: a pattern can be highly
    likely while its current impact is mild.
    """

    MILD = "轻度"
    MODERATE = "中度"
    SEVERE = "重度"


class EvidenceStrength(StrEnum):
    """Quality/completeness of supporting evidence, distinct from confidence."""

    HIGH = "高"
    MEDIUM = "中"
    LOW = "低"


DiagnosisAnalysisStatus = Literal[
    "completed",
    "partial",
    "insufficient_information",
    "safety_blocked",
]


class DiagnosisCandidateDraft(BaseModel):
    """One AI-proposed possibility before Go assigns durable identity.

    References use durable BodyState item IDs when the model can ground a claim to
    specific inputs. Empty reference lists are allowed during migration, but the
    schema now has an explicit place for traceable evidence instead of forcing all
    provenance into one prose ``basis`` field.
    """

    concern_key: str = "general"
    name: str = Field(min_length=1)
    confidence: DiagnosisConfidence
    severity: DiagnosisSeverity | None = None
    evidence_strength: EvidenceStrength | None = None
    impact: str | None = None

    basis: str = Field(default="")
    typical_symptoms: str = Field(default="")
    differential: str | None = None
    reasoning_summary: str = Field(default="")

    basis_fact_ids: list[str] = Field(default_factory=list)
    basis_observation_ids: list[str] = Field(default_factory=list)
    supporting_evidence_ids: list[str] = Field(default_factory=list)
    counterevidence_ids: list[str] = Field(default_factory=list)
    missing_information: list[str] = Field(default_factory=list)
    safety_notes: list[str] = Field(default_factory=list)


class DiagnosisAgentOutput(BaseModel):
    """Structured result of one Diagnosis run.

    There is intentionally **no max candidate count**. A long-lived user may have
    several active concerns and seven/eight candidates can be a valid result.

    Zero candidates are also valid when the status explicitly says why. A completed
    analysis, however, must contain at least one candidate; this keeps "success with
    no result" from becoming ambiguous state.
    """

    status: DiagnosisAnalysisStatus = "completed"
    scope: str = "full_body"
    summary: str = ""
    candidates: list[DiagnosisCandidateDraft] = Field(default_factory=list)
    cross_concern_patterns: list[str] = Field(default_factory=list)
    information_gaps: list[str] = Field(default_factory=list)
    safety_summary: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def validate_candidate_count_for_status(self) -> "DiagnosisAgentOutput":
        if self.status == "completed" and not self.candidates:
            raise ValueError("completed diagnosis analysis requires at least one candidate")
        return self


@dataclass(slots=True)
class DiagnosisDependencies:
    """Run-scoped input for one BodyState-based Diagnosis reasoning operation.

    Diagnosis reasoning consumes the durable state snapshot and temporal history
    that Go pinned for this run; transient consultation extraction is not a second
    health-truth input.
    """

    body_state_revision: int
    body_state: dict[str, Any]
    user_id: str = ""
    relevant_history: list[dict[str, Any]] = field(default_factory=list)
    profile: dict[str, Any] = field(default_factory=dict)
    rag_context: str = ""
    evidence_searcher: EvidenceSearcher | None = None
    retrieved_evidence: list[dict[str, Any]] = field(default_factory=list)
