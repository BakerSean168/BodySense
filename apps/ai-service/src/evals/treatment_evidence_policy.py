"""Deterministic Pydantic Evals suite for the Treatment EvidenceGap policy."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field
from pydantic_evals import Case, Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.agents.evidence import TreatmentEvidenceAcquirer
from src.models.evidence import EvidenceAttempt, EvidenceBudget, EvidenceGap

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


class TreatmentEvidencePolicyMetadata(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    expected_search_calls: int = Field(ge=0)
    expected_stop_reasons: list[str]
    expected_used_searches: int = Field(ge=0)
    expected_unresolved_critical_gap_ids: list[str]


class TreatmentEvidencePolicyOutput(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    search_calls: list[dict[str, Any]]
    attempts: list[EvidenceAttempt]
    budget: dict[str, int]
    unresolved_critical_gap_ids: list[str]


class _DeterministicSearcher:
    def __init__(self, results: dict[str, list[dict[str, Any]]]) -> None:
        self.results = results
        self.calls: list[dict[str, Any]] = []

    async def search(self, query: str, *, top_k: int = 5) -> list[dict[str, Any]]:
        self.calls.append({"query": query, "top_k": top_k})
        return list(self.results.get(query, []))[:top_k]


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
        searcher = _DeterministicSearcher(inputs.search_results)
        acquirer = TreatmentEvidenceAcquirer(
            searcher=searcher,
            budget=EvidenceBudget(
                max_searches=inputs.max_searches,
                max_results_per_search=inputs.max_results_per_search,
            ),
        )
        for gap in inputs.gaps:
            await acquirer.acquire(gap)
        trace = acquirer.trace()
        return TreatmentEvidencePolicyOutput(
            search_calls=searcher.calls,
            attempts=trace.attempts,
            budget=trace.budget,
            unresolved_critical_gap_ids=[gap.gap_id for gap in trace.unresolved_critical_gaps],
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
