"""Deterministic Pydantic Evals suite for the Treatment EvidenceGap policy."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field
from pydantic_evals import Case, Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.agents.evidence import TreatmentEvidenceAcquirer
from src.models.evidence import (
    EvidenceAttempt,
    EvidenceBudget,
    EvidenceGap,
    EvidenceRetrievalStatus,
    EvidenceSearchOutcome,
    ExternalEvidenceStatus,
)

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_TREATMENT_EVIDENCE_POLICY_DATASET_PATH = (
    SERVICE_ROOT / "data" / "evals" / "treatment_evidence_policy.yaml"
)


class TreatmentEvidencePolicyInputs(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    gaps: list[EvidenceGap]
    max_searches: int = Field(ge=0)
    max_results_per_search: int = Field(default=5, ge=1, le=10)
    search_results: dict[str, list[dict[str, Any]]] = Field(default_factory=dict)
    search_outcomes: dict[str, EvidenceRetrievalStatus] = Field(default_factory=dict)


class TreatmentEvidencePolicyMetadata(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    expected_search_calls: int = Field(ge=0)
    expected_stop_reasons: list[str]
    expected_used_searches: int = Field(ge=0)
    expected_unresolved_critical_gap_ids: list[str]
    expected_external_evidence_status: ExternalEvidenceStatus
    expected_returned_evidence_relations: list[str] = Field(default_factory=list)


class TreatmentEvidencePolicyOutput(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    search_calls: list[dict[str, Any]]
    attempts: list[EvidenceAttempt]
    budget: dict[str, int]
    unresolved_critical_gap_ids: list[str]
    external_evidence_status: ExternalEvidenceStatus
    returned_evidence_relations: list[str]


class _DeterministicSearcher:
    def __init__(
        self,
        results: dict[str, list[dict[str, Any]]],
        outcomes: dict[str, EvidenceRetrievalStatus],
    ) -> None:
        self.results = results
        self.outcomes = outcomes
        self.calls: list[dict[str, Any]] = []

    async def search(self, query: str, *, top_k: int = 5) -> EvidenceSearchOutcome:
        self.calls.append({"query": query, "top_k": top_k})
        results = list(self.results.get(query, []))[:top_k]
        forced = self.outcomes.get(query)
        if results:
            if forced not in (None, EvidenceRetrievalStatus.RESULTS_RETURNED):
                raise ValueError("forced non-result retrieval status cannot contain evidence")
            return EvidenceSearchOutcome(
                retrieval_status=EvidenceRetrievalStatus.RESULTS_RETURNED,
                evidence=results,
                published_corpus_count=max(1, len(results)),
            )
        if forced == EvidenceRetrievalStatus.PUBLISHED_CORPUS_EMPTY:
            return EvidenceSearchOutcome(
                retrieval_status=forced,
                published_corpus_count=0,
            )
        if forced == EvidenceRetrievalStatus.SEARCH_UNAVAILABLE:
            return EvidenceSearchOutcome(retrieval_status=forced)
        return EvidenceSearchOutcome(
            retrieval_status=EvidenceRetrievalStatus.NO_RELEVANT_RESULTS,
            published_corpus_count=1,
        )


EvalContext = EvaluatorContext[
    TreatmentEvidencePolicyInputs,
    TreatmentEvidencePolicyOutput,
    TreatmentEvidencePolicyMetadata,
]


@dataclass
class TreatmentEvidencePolicyBehavior(
    Evaluator[
        TreatmentEvidencePolicyInputs,
        TreatmentEvidencePolicyOutput,
        TreatmentEvidencePolicyMetadata,
    ]
):
    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        return (
            len(ctx.output.search_calls) == metadata.expected_search_calls
            and [attempt.stop_reason.value for attempt in ctx.output.attempts]
            == metadata.expected_stop_reasons
            and ctx.output.budget.get("used_searches") == metadata.expected_used_searches
            and ctx.output.unresolved_critical_gap_ids
            == metadata.expected_unresolved_critical_gap_ids
            and ctx.output.external_evidence_status == metadata.expected_external_evidence_status
            and ctx.output.returned_evidence_relations
            == metadata.expected_returned_evidence_relations
        )


def load_treatment_evidence_policy_dataset(
    path: Path = DEFAULT_TREATMENT_EVIDENCE_POLICY_DATASET_PATH,
) -> Dataset[
    TreatmentEvidencePolicyInputs,
    TreatmentEvidencePolicyOutput,
    TreatmentEvidencePolicyMetadata,
]:
    raw = Dataset[
        TreatmentEvidencePolicyInputs,
        TreatmentEvidencePolicyOutput,
        TreatmentEvidencePolicyMetadata,
    ].from_file(path)
    return Dataset(
        name=raw.name,
        cases=[
            Case(
                name=case.name,
                inputs=TreatmentEvidencePolicyInputs.model_validate(case.inputs),
                metadata=TreatmentEvidencePolicyMetadata.model_validate(case.metadata),
            )
            for case in raw.cases
        ],
        evaluators=[TreatmentEvidencePolicyBehavior()],
    )


def build_treatment_evidence_policy_task() -> Any:
    async def task(inputs: TreatmentEvidencePolicyInputs) -> TreatmentEvidencePolicyOutput:
        searcher = _DeterministicSearcher(inputs.search_results, inputs.search_outcomes)
        acquirer = TreatmentEvidenceAcquirer(
            searcher=searcher,
            budget=EvidenceBudget(
                max_searches=inputs.max_searches,
                max_results_per_search=inputs.max_results_per_search,
            ),
        )
        returned_evidence_relations: list[str] = []
        for gap in inputs.gaps:
            result = await acquirer.acquire(gap)
            returned_evidence_relations.extend(
                str(item.get("relation_to_hypothesis"))
                for item in result.evidence
                if item.get("relation_to_hypothesis")
            )
        trace = acquirer.trace()
        return TreatmentEvidencePolicyOutput(
            search_calls=searcher.calls,
            attempts=trace.attempts,
            budget=trace.budget,
            unresolved_critical_gap_ids=[gap.gap_id for gap in trace.unresolved_critical_gaps],
            external_evidence_status=trace.external_evidence_status,
            returned_evidence_relations=returned_evidence_relations,
        )

    return task


def run_treatment_evidence_policy_qualification(
    path: Path = DEFAULT_TREATMENT_EVIDENCE_POLICY_DATASET_PATH,
) -> Any:
    return load_treatment_evidence_policy_dataset(path).evaluate_sync(
        build_treatment_evidence_policy_task(),
        progress=False,
        name="treatment-evidence-policy-v2",
    )


def treatment_evidence_policy_summary(report: Any) -> dict[str, Any]:
    cases: list[dict[str, Any]] = []
    passed = 0
    for case in report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        cases.append({"name": case.name, "passed": case_passed, "assertions": assertions})
    return {
        "name": report.name,
        "passed": passed,
        "total": len(report.cases),
        "failed": len(report.cases) - passed,
        "cases": cases,
    }
