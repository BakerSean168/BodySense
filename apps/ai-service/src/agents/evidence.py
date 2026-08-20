"""Targeted evidence retrieval shared by Diagnosis and Treatment agents."""

from __future__ import annotations

import uuid
from dataclasses import dataclass, field
from typing import Any

from ..models.dependencies import EvidenceSearcher
from ..models.evidence import (
    EvidenceAcquisitionResult,
    EvidenceAcquisitionStatus,
    EvidenceAcquisitionTrace,
    EvidenceAttempt,
    EvidenceBudget,
    EvidenceGap,
    EvidenceGapKind,
    EvidenceStopReason,
)
from ..rag import get_knowledge_library


@dataclass(slots=True)
class KnowledgeEvidenceSearcher:
    """Adapter over the normalized knowledge library.

    Retrieval is invoked by an Agent tool only when the model identifies an
    evidence gap; Diagnosis no longer performs broad Go-side RAG on every run.
    """

    user_id: str

    async def search(self, query: str, *, top_k: int = 5) -> list[dict[str, Any]]:
        library = get_knowledge_library()
        results = await library.search(query=query, top_k=max(1, min(top_k, 10)))
        return [normalize_evidence(self.user_id, result) for result in results]


def normalize_evidence(user_id: str, result: Any) -> dict[str, Any]:
    source_type = "knowledge_unit"
    source_key = f"knowledge-unit:{result.id}"
    source_version = str(getattr(result, "source_timestamp", "") or "")
    identity = ":".join(["bodysense:evidence", user_id, source_type, source_key, source_version])
    evidence_id = str(uuid.uuid5(uuid.NAMESPACE_URL, identity))
    return {
        "evidence_id": evidence_id,
        "source_type": source_type,
        "source_key": source_key,
        "source_version": source_version,
        "title": result.title,
        "summary": result.summary,
        "excerpt": result.body_markdown,
        "problem_slug": result.problem_slug,
        "category": result.category,
        "unit_type": result.unit_type,
        "similarity": result.similarity,
        "source_title": result.source_title,
        "source_author": result.source_author,
        "source_timestamp": result.source_timestamp,
        "tags": result.tags,
    }


# Diagnosis v2 policy. Keep Treatment and the immutable Diagnosis v1 path on the
# existing EvidenceSearcher adapter until their own migrations explicitly cut over.
DIAGNOSIS_EVIDENCE_POLICY_V2 = "diagnosis-evidence-gap-v2"


@dataclass(slots=True)
class DiagnosisEvidenceAcquirer:
    """Policy-enforcing acquisition runtime for typed Diagnosis EvidenceGaps.

    The model may declare a gap, but this object owns whether an external search is
    legal and whether the per-run budget permits it. User facts can never be
    synthesized by RAG because ``user_fact`` is a hard no-search source class.
    """

    searcher: EvidenceSearcher | None
    budget: EvidenceBudget
    policy_revision: str = DIAGNOSIS_EVIDENCE_POLICY_V2
    attempts: list[EvidenceAttempt] = field(default_factory=list)

    def __post_init__(self) -> None:
        if self.policy_revision != DIAGNOSIS_EVIDENCE_POLICY_V2:
            raise ValueError(f"unsupported Diagnosis evidence policy: {self.policy_revision}")

    async def acquire(self, gap: EvidenceGap, *, top_k: int = 5) -> EvidenceAcquisitionResult:
        bounded_top_k = max(1, min(top_k, self.budget.max_results_per_search))

        if gap.kind == EvidenceGapKind.USER_FACT:
            attempt = EvidenceAttempt(
                gap=gap,
                status=EvidenceAcquisitionStatus.UNRESOLVED,
                stop_reason=EvidenceStopReason.USER_INPUT_REQUIRED,
                search_performed=False,
                query=None,
                requested_top_k=bounded_top_k,
            )
            return self._record(attempt)

        query = (gap.query or "").strip()
        if not self.budget.reserve_search():
            attempt = EvidenceAttempt(
                gap=gap,
                status=EvidenceAcquisitionStatus.UNRESOLVED,
                stop_reason=EvidenceStopReason.BUDGET_EXHAUSTED,
                search_performed=False,
                query=query,
                requested_top_k=bounded_top_k,
            )
            return self._record(attempt)

        if self.searcher is None:
            attempt = EvidenceAttempt(
                gap=gap,
                status=EvidenceAcquisitionStatus.UNRESOLVED,
                stop_reason=EvidenceStopReason.SEARCH_UNAVAILABLE,
                search_performed=False,
                query=query,
                requested_top_k=bounded_top_k,
            )
            return self._record(attempt)

        results = await self.searcher.search(query, top_k=bounded_top_k)
        evidence_ids = [
            str(item.get("evidence_id", "")) for item in results if item.get("evidence_id")
        ]
        attempt = EvidenceAttempt(
            gap=gap,
            status=(
                EvidenceAcquisitionStatus.EVIDENCE_RETURNED
                if results
                else EvidenceAcquisitionStatus.UNRESOLVED
            ),
            stop_reason=(
                EvidenceStopReason.EVIDENCE_RETURNED if results else EvidenceStopReason.NO_RESULTS
            ),
            search_performed=True,
            query=query,
            requested_top_k=bounded_top_k,
            evidence_ids=evidence_ids,
        )
        return self._record(attempt, evidence=results)

    def _record(
        self, attempt: EvidenceAttempt, *, evidence: list[dict[str, Any]] | None = None
    ) -> EvidenceAcquisitionResult:
        self.attempts.append(attempt)
        return EvidenceAcquisitionResult(
            attempt=attempt,
            evidence=evidence or [],
            budget=self.budget.snapshot(),
        )

    def trace(self) -> EvidenceAcquisitionTrace:
        unresolved = [
            attempt.gap
            for attempt in self.attempts
            if attempt.gap.critical and attempt.status == EvidenceAcquisitionStatus.UNRESOLVED
        ]
        return EvidenceAcquisitionTrace(
            policy_revision=self.policy_revision,
            budget=self.budget.snapshot(),
            attempts=list(self.attempts),
            unresolved_critical_gaps=unresolved,
        )
