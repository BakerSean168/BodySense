"""Deterministic qualification suite for Assessment evidence contract v2."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field
from pydantic_evals import Case, Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.services.assessment_evidence import (
    assessment_evidence_issues,
    build_assessment_evidence_catalog,
    build_assessment_evidence_coverage,
    render_assessment_observations,
)

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_ASSESSMENT_EVIDENCE_POLICY_DATASET_PATH = (
    SERVICE_ROOT / "data" / "evals" / "assessment_evidence_policy.yaml"
)


class AssessmentEvidencePolicyInputs(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    profile: dict[str, Any] = Field(default_factory=dict)
    body_state: dict[str, Any] = Field(default_factory=dict)
    report_indicators: list[Any] = Field(default_factory=list)
    posture_analysis: dict[str, Any] = Field(default_factory=dict)
    selections: list[dict[str, Any]] = Field(default_factory=list)


class AssessmentEvidencePolicyMetadata(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    expected_issue_count: int = Field(ge=0)
    expected_issue_fragments: list[str] = Field(default_factory=list)
    expected_coverage_status: str
    expected_available_domains: list[str] = Field(default_factory=list)
    expected_rendered_descriptions: list[str] = Field(default_factory=list)


class AssessmentEvidencePolicyOutput(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    issue_messages: list[str]
    coverage_status: str
    available_domains: list[str]
    rendered_descriptions: list[str]


EvalContext = EvaluatorContext[
    AssessmentEvidencePolicyInputs,
    AssessmentEvidencePolicyOutput,
    AssessmentEvidencePolicyMetadata,
]


@dataclass
class AssessmentEvidencePolicyBehavior(
    Evaluator[
        AssessmentEvidencePolicyInputs,
        AssessmentEvidencePolicyOutput,
        AssessmentEvidencePolicyMetadata,
    ]
):
    """Exact deterministic contract: gate, coverage, and source-only rendering."""

    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        return (
            len(ctx.output.issue_messages) == metadata.expected_issue_count
            and all(
                any(fragment in message for message in ctx.output.issue_messages)
                for fragment in metadata.expected_issue_fragments
            )
            and ctx.output.coverage_status == metadata.expected_coverage_status
            and ctx.output.available_domains == metadata.expected_available_domains
            and ctx.output.rendered_descriptions == metadata.expected_rendered_descriptions
        )


def load_assessment_evidence_policy_dataset(
    path: Path = DEFAULT_ASSESSMENT_EVIDENCE_POLICY_DATASET_PATH,
) -> Dataset[
    AssessmentEvidencePolicyInputs,
    AssessmentEvidencePolicyOutput,
    AssessmentEvidencePolicyMetadata,
]:
    raw = Dataset[
        AssessmentEvidencePolicyInputs,
        AssessmentEvidencePolicyOutput,
        AssessmentEvidencePolicyMetadata,
    ].from_file(path)
    return Dataset(
        name=raw.name,
        cases=[
            Case(
                name=case.name,
                inputs=AssessmentEvidencePolicyInputs.model_validate(case.inputs),
                metadata=AssessmentEvidencePolicyMetadata.model_validate(case.metadata),
            )
            for case in raw.cases
        ],
        evaluators=[AssessmentEvidencePolicyBehavior()],
    )


def build_assessment_evidence_policy_task() -> Any:
    async def task(inputs: AssessmentEvidencePolicyInputs) -> AssessmentEvidencePolicyOutput:
        catalog = build_assessment_evidence_catalog(
            profile=inputs.profile,
            body_state=inputs.body_state,
            report_indicators=inputs.report_indicators,
            posture_analysis=inputs.posture_analysis,
        )
        issues = assessment_evidence_issues({"observations": inputs.selections}, catalog)
        coverage = build_assessment_evidence_coverage(catalog)
        domains = coverage.get("domains") or {}
        available_domains = sorted(
            name
            for name, value in domains.items()
            if isinstance(value, dict) and value.get("status") == "available"
        )
        rendered: list[dict[str, Any]] = []
        if not issues:
            rendered = render_assessment_observations(inputs.selections, catalog)
        return AssessmentEvidencePolicyOutput(
            issue_messages=[issue.message for issue in issues],
            coverage_status=str(coverage.get("status") or ""),
            available_domains=available_domains,
            rendered_descriptions=[str(item.get("description") or "") for item in rendered],
        )

    return task


def run_assessment_evidence_policy_qualification(
    path: Path = DEFAULT_ASSESSMENT_EVIDENCE_POLICY_DATASET_PATH,
) -> Any:
    return load_assessment_evidence_policy_dataset(path).evaluate_sync(
        build_assessment_evidence_policy_task(),
        progress=False,
        name="assessment-evidence-contract-v2",
    )


def assessment_evidence_policy_summary(report: Any) -> dict[str, Any]:
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
