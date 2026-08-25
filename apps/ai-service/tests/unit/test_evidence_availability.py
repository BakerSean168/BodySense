from __future__ import annotations

import pytest

import src.agents.evidence as evidence_module
from src.agents.evidence import DiagnosisEvidenceAcquirer, KnowledgeEvidenceSearcher
from src.models.evidence import (
    EvidenceBudget,
    EvidenceGap,
    EvidenceGapKind,
    EvidenceRetrievalStatus,
    EvidenceSearchOutcome,
    EvidenceStopReason,
    ExternalEvidenceStatus,
)
from src.rag.knowledge_library import KnowledgeLibraryUnavailableError, SearchResult


class StaticOutcomeSearcher:
    def __init__(self, *outcomes: EvidenceSearchOutcome) -> None:
        self.outcomes = list(outcomes)
        self.calls: list[tuple[str, int]] = []

    async def search(self, query: str, *, top_k: int = 5) -> EvidenceSearchOutcome:
        self.calls.append((query, top_k))
        return self.outcomes.pop(0)


def external_gap(gap_id: str) -> EvidenceGap:
    return EvidenceGap(
        gap_id=gap_id,
        kind=EvidenceGapKind.EXTERNAL_KNOWLEDGE,
        description=f"external gap {gap_id}",
        rationale="changes reasoning",
        query=f"query {gap_id}",
    )


def returned_evidence(evidence_id: str = "evidence-1") -> EvidenceSearchOutcome:
    return EvidenceSearchOutcome(
        retrieval_status=EvidenceRetrievalStatus.RESULTS_RETURNED,
        evidence=[
            {"evidence_id": evidence_id, "content": "retrieved but not automatically support"}
        ],
        published_corpus_count=3,
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("outcome", "expected_stop", "search_performed"),
    [
        (
            EvidenceSearchOutcome(
                retrieval_status=EvidenceRetrievalStatus.PUBLISHED_CORPUS_EMPTY,
                published_corpus_count=0,
            ),
            EvidenceStopReason.PUBLISHED_CORPUS_EMPTY,
            True,
        ),
        (
            EvidenceSearchOutcome(
                retrieval_status=EvidenceRetrievalStatus.NO_RELEVANT_RESULTS,
                published_corpus_count=3,
            ),
            EvidenceStopReason.NO_RELEVANT_RESULTS,
            True,
        ),
        (
            EvidenceSearchOutcome(
                retrieval_status=EvidenceRetrievalStatus.SEARCH_UNAVAILABLE,
            ),
            EvidenceStopReason.SEARCH_UNAVAILABLE,
            True,
        ),
    ],
)
async def test_acquisition_distinguishes_empty_irrelevant_and_unavailable(
    outcome: EvidenceSearchOutcome,
    expected_stop: EvidenceStopReason,
    search_performed: bool,
) -> None:
    acquirer = DiagnosisEvidenceAcquirer(
        searcher=StaticOutcomeSearcher(outcome),
        budget=EvidenceBudget(max_searches=2),
    )
    result = await acquirer.acquire(external_gap("g1"))
    trace = acquirer.trace()

    assert result.attempt.stop_reason == expected_stop
    assert result.attempt.retrieval_status == outcome.retrieval_status
    assert result.attempt.search_performed is search_performed
    assert trace.external_evidence_status == ExternalEvidenceStatus.UNRESOLVED


@pytest.mark.asyncio
async def test_trace_distinguishes_not_required_available_and_partial() -> None:
    user_only = DiagnosisEvidenceAcquirer(searcher=None, budget=EvidenceBudget())
    await user_only.acquire(
        EvidenceGap(
            gap_id="user-fact",
            kind=EvidenceGapKind.USER_FACT,
            description="user fact",
            rationale="must ask user",
        )
    )
    assert user_only.trace().external_evidence_status == ExternalEvidenceStatus.NOT_REQUIRED

    all_available = DiagnosisEvidenceAcquirer(
        searcher=StaticOutcomeSearcher(returned_evidence()), budget=EvidenceBudget(max_searches=2)
    )
    await all_available.acquire(external_gap("g1"))
    assert all_available.trace().external_evidence_status == ExternalEvidenceStatus.AVAILABLE

    partial = DiagnosisEvidenceAcquirer(
        searcher=StaticOutcomeSearcher(
            returned_evidence(),
            EvidenceSearchOutcome(
                retrieval_status=EvidenceRetrievalStatus.NO_RELEVANT_RESULTS,
                published_corpus_count=3,
            ),
        ),
        budget=EvidenceBudget(max_searches=2),
    )
    await partial.acquire(external_gap("g1"))
    await partial.acquire(external_gap("g2"))
    assert partial.trace().external_evidence_status == ExternalEvidenceStatus.PARTIALLY_AVAILABLE


class FakeKnowledgeLibrary:
    def __init__(
        self,
        *,
        count: int = 0,
        results: list[SearchResult] | None = None,
        error: Exception | None = None,
    ) -> None:
        self.count = count
        self.results = results or []
        self.error = error
        self.search_calls = 0

    async def published_corpus_count(self) -> int:
        if self.error is not None:
            raise self.error
        return self.count

    async def search(self, **_: object) -> list[SearchResult]:
        self.search_calls += 1
        if self.error is not None:
            raise self.error
        return self.results


def sample_search_result() -> SearchResult:
    return SearchResult(
        id=1,
        problem_slug="pain-science",
        category="definition",
        unit_type="reference",
        title="Pain",
        summary="Pain definition",
        body_markdown="Pain is not identical to nociception.",
        similarity=0.9,
        source_title="Pain",
        source_author="Thought Forest",
        source_start_sec=0,
        source_end_sec=0,
        source_type="thought_forest_note",
        source_key="thought-forest:z/pain.md",
        unit_key="pain-definition",
        lifecycle_status="published",
        review_status="reviewed",
        quality_score=0.95,
        publication_id="publication-1",
        published_version=1,
    )


@pytest.mark.asyncio
async def test_knowledge_evidence_searcher_checks_corpus_before_embedding_search(
    monkeypatch,
) -> None:
    library = FakeKnowledgeLibrary(count=0)
    monkeypatch.setattr(evidence_module, "get_knowledge_library", lambda: library)

    outcome = await KnowledgeEvidenceSearcher("user-1").search("pain", top_k=5)

    assert outcome.retrieval_status == EvidenceRetrievalStatus.PUBLISHED_CORPUS_EMPTY
    assert outcome.published_corpus_count == 0
    assert library.search_calls == 0


@pytest.mark.asyncio
async def test_knowledge_evidence_searcher_distinguishes_irrelevant_and_available(
    monkeypatch,
) -> None:
    library = FakeKnowledgeLibrary(count=3, results=[])
    monkeypatch.setattr(evidence_module, "get_knowledge_library", lambda: library)
    irrelevant = await KnowledgeEvidenceSearcher("user-1").search("unrelated", top_k=5)
    assert irrelevant.retrieval_status == EvidenceRetrievalStatus.NO_RELEVANT_RESULTS

    library.results = [sample_search_result()]
    available = await KnowledgeEvidenceSearcher("user-1").search("pain", top_k=5)
    assert available.retrieval_status == EvidenceRetrievalStatus.RESULTS_RETURNED
    assert len(available.evidence) == 1
    assert available.evidence[0]["publication_id"] == "publication-1"


@pytest.mark.asyncio
async def test_knowledge_evidence_searcher_degrades_known_backend_failure(monkeypatch) -> None:
    library = FakeKnowledgeLibrary(error=KnowledgeLibraryUnavailableError("database unavailable"))
    monkeypatch.setattr(evidence_module, "get_knowledge_library", lambda: library)

    outcome = await KnowledgeEvidenceSearcher("user-1").search("pain", top_k=5)

    assert outcome.retrieval_status == EvidenceRetrievalStatus.SEARCH_UNAVAILABLE
    assert outcome.evidence == []


def test_search_outcome_does_not_claim_semantic_support() -> None:
    outcome = returned_evidence()
    contradictory = outcome.model_copy(
        update={
            "evidence": [
                {
                    "evidence_id": "contra-1",
                    "content": "This evidence contradicts the candidate.",
                    "relation_to_hypothesis": "contradicts",
                }
            ]
        },
    )
    assert contradictory.retrieval_status == EvidenceRetrievalStatus.RESULTS_RETURNED
    assert contradictory.evidence[0]["relation_to_hypothesis"] == "contradicts"
