"""Pydantic Evals characterization harness for the production Diagnosis application path."""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from pydantic import BaseModel, Field
from pydantic_evals import Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.agents.diagnosis_agent import create_diagnosis_agent
from src.configuration.diagnosis_agent_config import get_default_diagnosis_configuration
from src.services.diagnosis_service import DiagnosisService
from src.testing_support.deterministic_ai import deterministic_diagnosis_model

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_DATASET_PATH = SERVICE_ROOT / "data" / "evals" / "diagnosis_baseline.yaml"


class DiagnosisEvalInputs(BaseModel):
    """Frozen application input for one Diagnosis characterization case."""

    user_id: str = "eval-user"
    body_state_revision: int = Field(gt=0)
    body_state: dict[str, Any]
    relevant_history: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)


class DiagnosisEvalMetadata(BaseModel):
    """Case taxonomy and deterministic expectations used by baseline evaluators."""

    scenario_family_id: str
    case_category: str
    risk_slice: str
    expected_status: str
    min_candidates: int = 0
    max_candidates: int | None = None
    required_concern_keys: list[str] = Field(default_factory=list)
    forbidden_output_fields: list[str] = Field(
        default_factory=lambda: ["treatment", "training_plan"]
    )


@dataclass
class DiagnosisContract(Evaluator[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata]):
    """Protect the stable Diagnosis response boundary independent of wording."""

    def evaluate(
        self,
        ctx: EvaluatorContext[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata],
    ) -> bool:
        output = ctx.output
        return (
            isinstance(output, dict)
            and isinstance(output.get("status"), str)
            and isinstance(output.get("candidates"), list)
            and isinstance(output.get("governance"), dict)
            and output["governance"].get("kind") == "diagnosis"
            and output["governance"].get("verdict") in {"accepted", "degraded", "rejected"}
        )


@dataclass
class ExpectedStatus(Evaluator[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata]):
    """Assert the case-specific top-level Diagnosis status."""

    def evaluate(
        self,
        ctx: EvaluatorContext[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata],
    ) -> bool:
        metadata = ctx.metadata
        return metadata is not None and ctx.output.get("status") == metadata.expected_status


@dataclass
class CandidatePolicy(Evaluator[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata]):
    """Assert candidate cardinality and required concern coverage."""

    def evaluate(
        self,
        ctx: EvaluatorContext[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata],
    ) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        candidates = ctx.output.get("candidates")
        if not isinstance(candidates, list):
            return False
        if len(candidates) < metadata.min_candidates:
            return False
        if metadata.max_candidates is not None and len(candidates) > metadata.max_candidates:
            return False
        actual_concerns = {
            str(candidate.get("concern_key"))
            for candidate in candidates
            if isinstance(candidate, dict) and candidate.get("concern_key")
        }
        return all(key in actual_concerns for key in metadata.required_concern_keys)


@dataclass
class NoTreatmentSideEffect(
    Evaluator[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata]
):
    """Diagnosis must not silently expand into Treatment/Training output."""

    def evaluate(
        self,
        ctx: EvaluatorContext[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata],
    ) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        return not any(field in ctx.output for field in metadata.forbidden_output_fields)


def load_diagnosis_dataset(path: Path = DEFAULT_DATASET_PATH) -> Dataset[
    DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata
]:
    """Load the versioned YAML cases and attach code-versioned deterministic evaluators."""

    cases = Dataset[DiagnosisEvalInputs, dict[str, Any], DiagnosisEvalMetadata].from_file(path)
    return Dataset(
        name=cases.name,
        cases=cases.cases,
        evaluators=[
            DiagnosisContract(),
            ExpectedStatus(),
            CandidatePolicy(),
            NoTreatmentSideEffect(),
        ],
    )


def build_deterministic_task() -> Any:
    """Return an async task that exercises the same Diagnosis application path as production."""

    config = get_default_diagnosis_configuration()
    service = DiagnosisService(
        diagnosis_agent=create_diagnosis_agent(deterministic_diagnosis_model())
    )

    async def task(inputs: DiagnosisEvalInputs) -> dict[str, Any]:
        return await service.generate_diagnosis(
            user_id=inputs.user_id,
            body_state_revision=inputs.body_state_revision,
            configuration_id=config.configuration_id,
            body_state=inputs.body_state,
            relevant_history=inputs.relevant_history,
            profile=inputs.profile,
        )

    return task


def run_diagnosis_baseline(path: Path = DEFAULT_DATASET_PATH) -> Any:
    """Evaluate the deterministic current-behavior baseline."""

    dataset = load_diagnosis_dataset(path)
    return dataset.evaluate_sync(
        build_deterministic_task(), progress=False, name="diagnosis-baseline"
    )


def report_summary(report: Any) -> dict[str, Any]:
    """Convert the Pydantic Evals report into a stable CI-friendly summary."""

    cases: list[dict[str, Any]] = []
    passed = 0
    slice_totals: dict[str, dict[str, int]] = {}
    for case in report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        risk_slice = str(getattr(case.metadata, "risk_slice", "unknown"))
        bucket = slice_totals.setdefault(risk_slice, {"passed": 0, "total": 0})
        bucket["total"] += 1
        bucket["passed"] += int(case_passed)
        cases.append(
            {
                "name": case.name,
                "passed": case_passed,
                "risk_slice": risk_slice,
                "assertions": assertions,
            }
        )
    config = get_default_diagnosis_configuration()
    return {
        "name": report.name,
        "configuration_id": config.configuration_id,
        "configuration": config.provenance(),
        "passed": passed,
        "total": len(report.cases),
        "failed": len(report.cases) - passed,
        "slices": slice_totals,
        "cases": cases,
    }


def render_summary(report: Any) -> str:
    """Render a compact human-readable baseline report."""

    summary = report_summary(report)
    lines = [
        "# Diagnosis Pydantic Evals Baseline",
        "",
        f"- Result: {summary['passed']}/{summary['total']} passed",
        f"- Agent configuration: `{summary['configuration_id']}`",
        "- Mode: deterministic production-path characterization",
        "",
        "## Risk slices",
    ]
    for name, stats in sorted(summary["slices"].items()):
        lines.append(f"- `{name}`: {stats['passed']}/{stats['total']} passed")
    lines.extend(["", "## Cases"])
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        lines.append(f"- `{mark}` `{case['name']}` ({case['risk_slice']})")
    return "\n".join(lines) + "\n"


def summary_json(report: Any) -> str:
    return json.dumps(report_summary(report), ensure_ascii=False, indent=2) + "\n"
