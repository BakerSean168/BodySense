"""Deterministic qualification for the v5 Diagnosis negation safety boundary."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict
from pydantic_evals import Case, Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.services.red_flag_detector import RedFlagDetector

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_NEGATION_POLICY_DATASET_PATH = (
    SERVICE_ROOT / "data" / "evals" / "diagnosis_negation_policy_v2.yaml"
)

DIAGNOSIS_V5_CONFIGURATION_ID = "diag-config-375187050b203078"
DIAGNOSIS_V5_GOVERNANCE_POLICY_REVISION = "diagnosis-governance-v5-negation-aware-claims"
DIAGNOSIS_V5_DETECTOR_REVISION = "red-flag-detector-negation-aware-v2"


class NegationPolicyEvalInputs(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    text: str
    revision: str


class NegationPolicyEvalMetadata(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    expected_categories: list[str]


class NegationPolicyEvalOutput(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    categories: list[str]


EvalContext = EvaluatorContext[
    NegationPolicyEvalInputs, NegationPolicyEvalOutput, NegationPolicyEvalMetadata
]


@dataclass
class NegationPolicyBehavior(
    Evaluator[NegationPolicyEvalInputs, NegationPolicyEvalOutput, NegationPolicyEvalMetadata]
):
    """Require exact category outcomes for each immutable policy boundary case."""

    def evaluate(self, ctx: EvalContext) -> bool:
        return (
            ctx.metadata is not None
            and ctx.output.categories == ctx.metadata.expected_categories
        )


def load_negation_policy_dataset(
    path: Path = DEFAULT_NEGATION_POLICY_DATASET_PATH,
) -> Dataset[NegationPolicyEvalInputs, NegationPolicyEvalOutput, NegationPolicyEvalMetadata]:
    raw = Dataset[
        NegationPolicyEvalInputs, NegationPolicyEvalOutput, NegationPolicyEvalMetadata
    ].from_file(path)
    return Dataset(
        name=raw.name,
        cases=[
            Case(
                name=case.name,
                inputs=NegationPolicyEvalInputs.model_validate(case.inputs),
                metadata=NegationPolicyEvalMetadata.model_validate(case.metadata),
            )
            for case in raw.cases
        ],
        evaluators=[NegationPolicyBehavior()],
    )


def build_negation_policy_task() -> Any:
    def task(inputs: NegationPolicyEvalInputs) -> NegationPolicyEvalOutput:
        result = RedFlagDetector().detect([], inputs.text, revision=inputs.revision)
        return NegationPolicyEvalOutput(
            categories=sorted(flag.category for flag in result.flags),
        )

    return task


def run_negation_policy_qualification(
    path: Path = DEFAULT_NEGATION_POLICY_DATASET_PATH,
) -> Any:
    return load_negation_policy_dataset(path).evaluate_sync(
        build_negation_policy_task(),
        progress=False,
        name="diagnosis-negation-policy-v2",
    )


def negation_policy_summary(report: Any) -> dict[str, Any]:
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
        "configuration_id": DIAGNOSIS_V5_CONFIGURATION_ID,
        "governance_policy_revision": DIAGNOSIS_V5_GOVERNANCE_POLICY_REVISION,
        "detector_revision": DIAGNOSIS_V5_DETECTOR_REVISION,
    }
