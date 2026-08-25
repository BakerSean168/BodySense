"""Typed evidence-gap and bounded acquisition contracts for Agent reasoning."""

from __future__ import annotations

from enum import StrEnum

from pydantic import BaseModel, ConfigDict, Field, model_validator


class EvidenceGapKind(StrEnum):
    """Where missing information is allowed to come from."""

    USER_FACT = "user_fact"
    EXTERNAL_KNOWLEDGE = "external_knowledge"


class EvidenceAcquisitionStatus(StrEnum):
    """Outcome of one evidence-gap acquisition attempt."""

    EVIDENCE_RETURNED = "evidence_returned"
    UNRESOLVED = "unresolved"


class EvidenceRetrievalStatus(StrEnum):
    """Observable state of the external Knowledge retrieval boundary."""

    RESULTS_RETURNED = "results_returned"
    PUBLISHED_CORPUS_EMPTY = "published_corpus_empty"
    NO_RELEVANT_RESULTS = "no_relevant_results"
    SEARCH_UNAVAILABLE = "search_unavailable"


class ExternalEvidenceStatus(StrEnum):
    """Run-level availability of externally requested evidence.

    This is deliberately about availability, not semantic support. Retrieved
    evidence may still contradict a hypothesis and must be interpreted by the
    Agent/grounding layer rather than being counted as support automatically.
    """

    NOT_REQUIRED = "not_required"
    AVAILABLE = "available"
    PARTIALLY_AVAILABLE = "partially_available"
    UNRESOLVED = "unresolved"


class EvidenceStopReason(StrEnum):
    """Why acquisition stopped for a gap."""

    EVIDENCE_RETURNED = "evidence_returned"
    USER_INPUT_REQUIRED = "user_input_required"
    BUDGET_EXHAUSTED = "budget_exhausted"
    SEARCH_UNAVAILABLE = "search_unavailable"
    PUBLISHED_CORPUS_EMPTY = "published_corpus_empty"
    NO_RELEVANT_RESULTS = "no_relevant_results"
    NO_RESULTS = "no_results"  # legacy persisted trace compatibility


class EvidenceGap(BaseModel):
    """A concrete information deficit identified during Agent reasoning.

    ``user_fact`` gaps are never eligible for external retrieval. An
    ``external_knowledge`` gap must include a targeted query and a rationale for why
    acquiring that evidence can materially change the governed Agent output.
    """

    model_config = ConfigDict(frozen=True, extra="forbid")

    gap_id: str = Field(min_length=1, max_length=80)
    kind: EvidenceGapKind
    description: str = Field(min_length=1, max_length=500)
    rationale: str = Field(min_length=1, max_length=500)
    critical: bool = False
    query: str | None = Field(default=None, max_length=500)

    @model_validator(mode="after")
    def validate_source_contract(self) -> "EvidenceGap":
        query = (self.query or "").strip()
        if self.kind == EvidenceGapKind.EXTERNAL_KNOWLEDGE and not query:
            raise ValueError("external_knowledge EvidenceGap requires a targeted query")
        if self.kind == EvidenceGapKind.USER_FACT and query:
            raise ValueError("user_fact EvidenceGap must not contain an external search query")
        return self


class EvidenceBudget(BaseModel):
    """Finite per-run evidence acquisition budget."""

    model_config = ConfigDict(extra="forbid")

    max_searches: int = Field(default=2, ge=0, le=10)
    max_results_per_search: int = Field(default=5, ge=1, le=10)
    used_searches: int = Field(default=0, ge=0)

    @property
    def remaining_searches(self) -> int:
        return max(0, self.max_searches - self.used_searches)

    def reserve_search(self) -> bool:
        if self.remaining_searches <= 0:
            return False
        self.used_searches += 1
        return True

    def snapshot(self) -> dict[str, int]:
        return {
            "max_searches": self.max_searches,
            "max_results_per_search": self.max_results_per_search,
            "used_searches": self.used_searches,
            "remaining_searches": self.remaining_searches,
        }


class EvidenceSearchOutcome(BaseModel):
    """Typed result returned by an EvidenceSearcher boundary."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    retrieval_status: EvidenceRetrievalStatus
    evidence: list[dict[str, object]] = Field(default_factory=list)
    published_corpus_count: int | None = Field(default=None, ge=0)

    @model_validator(mode="after")
    def validate_status_payload(self) -> "EvidenceSearchOutcome":
        if self.retrieval_status == EvidenceRetrievalStatus.RESULTS_RETURNED:
            if not self.evidence:
                raise ValueError("results_returned requires evidence")
        elif self.evidence:
            raise ValueError("non-result retrieval status must not contain evidence")
        if (
            self.retrieval_status == EvidenceRetrievalStatus.PUBLISHED_CORPUS_EMPTY
            and self.published_corpus_count != 0
        ):
            raise ValueError("published_corpus_empty requires published_corpus_count=0")
        return self


class EvidenceAttempt(BaseModel):
    """Structured audit record for one requested EvidenceGap."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    gap: EvidenceGap
    status: EvidenceAcquisitionStatus
    stop_reason: EvidenceStopReason
    search_performed: bool
    retrieval_status: EvidenceRetrievalStatus | None = None
    query: str | None = None
    requested_top_k: int = Field(ge=1, le=10)
    evidence_ids: list[str] = Field(default_factory=list)


class EvidenceAcquisitionResult(BaseModel):
    """Tool-visible result of one controlled acquisition attempt."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    attempt: EvidenceAttempt
    evidence: list[dict[str, object]] = Field(default_factory=list)
    budget: dict[str, int]


class EvidenceAcquisitionTrace(BaseModel):
    """Run-level acquisition trace emitted as non-authoritative execution metadata."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    trace_revision: str = "evidence-acquisition-trace-v2"
    policy_revision: str
    external_evidence_status: ExternalEvidenceStatus
    budget: dict[str, int]
    attempts: list[EvidenceAttempt] = Field(default_factory=list)
    unresolved_critical_gaps: list[EvidenceGap] = Field(default_factory=list)
