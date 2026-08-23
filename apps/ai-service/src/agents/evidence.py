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
    result_source_type = str(getattr(result, "source_type", "") or "")
    if result_source_type == "thought_forest_note":
        unit_metadata = dict(getattr(result, "unit_metadata", {}) or {})
        source_metadata = dict(getattr(result, "source_metadata", {}) or {})
        locator = dict(unit_metadata.get("source_locator") or {})
        claim_candidate = dict(unit_metadata.get("claim_candidate") or {})
        repository = dict(source_metadata.get("repository") or {})
        git_commit = str(locator.get("git_commit") or repository.get("git_commit") or "")
        section_hash = str(unit_metadata.get("section_content_hash") or "")
        stable_source_key = str(getattr(result, "source_key", "") or "")
        unit_key = str(getattr(result, "unit_key", "") or "")
        if stable_source_key and unit_key:
            source_key = f"{stable_source_key}#{unit_key}"
        else:
            source_key = stable_source_key or f"knowledge-unit:{result.id}"
        if git_commit and section_hash:
            source_version = f"{git_commit}:{section_hash}"
        else:
            source_version = git_commit or section_hash or str(
                getattr(result, "source_timestamp", "") or ""
            )
        source_type = "thought_forest_note"
    else:
        source_type = "knowledge_unit"
        source_key = f"knowledge-unit:{result.id}"
        source_version = str(getattr(result, "source_timestamp", "") or "")
        locator = {}
        claim_candidate = {}

    identity = ":".join(["bodysense:evidence", user_id, source_type, source_key, source_version])
    evidence_id = str(uuid.uuid5(uuid.NAMESPACE_URL, identity))
    evidence = {
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
    if locator:
        evidence["source_locator"] = locator
    if claim_candidate:
        evidence["claim_candidate"] = claim_candidate
        for key in (
            "claim_id",
            "claim_kind",
            "authority_tier",
            "certainty",
            "evidence_level",
            "external_evidence_status",
            "population",
        ):
            if key in claim_candidate:
                evidence[key] = claim_candidate[key]
    return evidence


DIAGNOSIS_EVIDENCE_POLICY_V2 = "diagnosis-evidence-gap-v2"
TREATMENT_EVIDENCE_POLICY_V2 = "treatment-evidence-gap-v2"


@dataclass(slots=True)
class _BoundedEvidenceAcquirer:
    """Shared policy engine for one Agent run's typed EvidenceGaps."""

    searcher: EvidenceSearcher | None
    budget: EvidenceBudget
    policy_revision: str
    attempts: list[EvidenceAttempt] = field(default_factory=list)

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


@dataclass(slots=True)
class DiagnosisEvidenceAcquirer(_BoundedEvidenceAcquirer):
    """Diagnosis v2 wrapper over the shared bounded acquisition engine."""

    policy_revision: str = DIAGNOSIS_EVIDENCE_POLICY_V2

    def __post_init__(self) -> None:
        if self.policy_revision != DIAGNOSIS_EVIDENCE_POLICY_V2:
            raise ValueError(f"unsupported Diagnosis evidence policy: {self.policy_revision}")


@dataclass(slots=True)
class TreatmentEvidenceAcquirer(_BoundedEvidenceAcquirer):
    """Treatment v2 wrapper over the shared bounded acquisition engine."""

    policy_revision: str = TREATMENT_EVIDENCE_POLICY_V2

    def __post_init__(self) -> None:
        if self.policy_revision != TREATMENT_EVIDENCE_POLICY_V2:
            raise ValueError(f"unsupported Treatment evidence policy: {self.policy_revision}")
