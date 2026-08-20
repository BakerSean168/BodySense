"""Pydantic Evals qualification system for immutable Diagnosis Agent configurations."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field
from pydantic_ai import ToolCallPart, capture_run_messages
from pydantic_evals import Case, Dataset
from pydantic_evals.evaluators import Evaluator, EvaluatorContext

from src.agents.diagnosis_agent import create_diagnosis_agent
from src.configuration.diagnosis_agent_config import (
    DiagnosisAgentManifest,
    get_default_diagnosis_configuration,
    get_diagnosis_configuration,
)
from src.services.diagnosis_service import DiagnosisService
from src.testing_support.deterministic_ai import deterministic_diagnosis_model

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_DATASET_PATH = SERVICE_ROOT / "data" / "evals" / "diagnosis_qualification.yaml"
DATASET_SCHEMA_PATH = SERVICE_ROOT / "data" / "evals" / "diagnosis_qualification.schema.json"
DEFAULT_CHAMPION_REPORT_PATH = (
    SERVICE_ROOT / "data" / "evals" / "reports" / "diagnosis_champion_baseline.json"
)

EvalSplit = Literal["development", "holdout", "regression", "challenge"]

DEFAULT_QUALIFICATION_POLICY: dict[str, Any] = {
    "required_splits": ["development", "holdout", "regression", "challenge"],
    "minimum_overall_pass_rate": 1.0,
    "critical_slices": ["critical-safety"],
    "minimum_critical_slice_pass_rate": 1.0,
    "non_inferiority_margin": 0.02,
}


class DiagnosisEvalInputs(BaseModel):
    """Frozen production-shaped input for one Diagnosis qualification case."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    user_id: str = "eval-user"
    body_state_revision: int = Field(gt=0)
    body_state: dict[str, Any]
    relevant_history: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)


class DiagnosisEvalMetadata(BaseModel):
    """Case taxonomy plus deterministic hard expectations used for qualification."""

    model_config = ConfigDict(frozen=True, extra="forbid")

    scenario_family_id: str = Field(min_length=1)
    case_category: str = Field(min_length=1)
    split: EvalSplit
    slices: list[str] = Field(min_length=1)
    critical: bool = False
    expected_status: str = Field(min_length=1)
    expected_agent_executed: bool = True
    expected_available_tools: list[str] = Field(default_factory=lambda: ["search_evidence"])
    max_tool_calls: int = Field(default=0, ge=0)
    min_candidates: int = Field(default=0, ge=0)
    max_candidates: int | None = Field(default=None, ge=0)
    required_concern_keys: list[str] = Field(default_factory=list)
    forbidden_output_fields: list[str] = Field(
        default_factory=lambda: ["treatment", "training_plan"]
    )


class DiagnosisEvalCaseDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    inputs: DiagnosisEvalInputs
    metadata: DiagnosisEvalMetadata


class DiagnosisDatasetDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    cases: list[DiagnosisEvalCaseDocument] = Field(min_length=1)


class DiagnosisEvalTrace(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    agent_executed: bool
    available_tools: list[str] = Field(default_factory=list)
    tool_calls: list[str] = Field(default_factory=list)


class DiagnosisEvalExecution(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    payload: dict[str, Any]
    configuration_id: str
    trace: DiagnosisEvalTrace


@dataclass(frozen=True)
class DiagnosisQualificationRun:
    report: Any
    dataset_path: Path
    dataset_fingerprint: str
    configuration: DiagnosisAgentManifest
    mode: Literal["deterministic"] = "deterministic"


EvalContext = EvaluatorContext[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]


@dataclass
class DiagnosisContract(
    Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
):
    """Protect the stable Diagnosis response boundary independent of wording."""

    def evaluate(self, ctx: EvalContext) -> bool:
        output = ctx.output.payload
        return (
            isinstance(output, dict)
            and isinstance(output.get("status"), str)
            and isinstance(output.get("candidates"), list)
            and isinstance(output.get("governance"), dict)
            and output["governance"].get("kind") == "diagnosis"
            and output["governance"].get("verdict") in {"accepted", "degraded", "rejected"}
        )


@dataclass
class ExpectedStatus(Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]):
    """Assert the case-specific top-level Diagnosis status."""

    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        return metadata is not None and ctx.output.payload.get("status") == metadata.expected_status


@dataclass
class CandidatePolicy(
    Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
):
    """Assert candidate cardinality and required concern coverage."""

    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        candidates = ctx.output.payload.get("candidates")
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
    Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
):
    """Diagnosis must not silently expand into Treatment/Training output."""

    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        return not any(field in ctx.output.payload for field in metadata.forbidden_output_fields)


@dataclass
class ConfigurationProvenance(
    Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
):
    """The runtime must report the exact immutable configuration that was evaluated."""

    def evaluate(self, ctx: EvalContext) -> bool:
        configuration = ctx.output.payload.get("agent_configuration")
        return (
            isinstance(configuration, dict)
            and configuration.get("id") == ctx.output.configuration_id
            and configuration.get("role") == "diagnosis"
        )


@dataclass
class ToolTracePolicy(
    Evaluator[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]
):
    """Grade Agent execution bypass, exposed tool surface, and bounded tool-call traces."""

    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        trace = ctx.output.trace
        if trace.agent_executed != metadata.expected_agent_executed:
            return False
        if sorted(trace.available_tools) != sorted(metadata.expected_available_tools):
            return False
        if len(trace.tool_calls) > metadata.max_tool_calls:
            return False
        return all(name in metadata.expected_available_tools for name in trace.tool_calls)


def load_dataset_document(path: Path = DEFAULT_DATASET_PATH) -> DiagnosisDatasetDocument:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    return DiagnosisDatasetDocument.model_validate(raw)


def dataset_fingerprint(document: DiagnosisDatasetDocument) -> str:
    canonical = json.dumps(
        document.model_dump(mode="json"),
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(canonical.encode()).hexdigest()


def dataset_schema_json() -> str:
    return (
        json.dumps(
            DiagnosisDatasetDocument.model_json_schema(),
            ensure_ascii=False,
            indent=2,
            sort_keys=True,
        )
        + "\n"
    )


def load_diagnosis_dataset(
    path: Path = DEFAULT_DATASET_PATH,
) -> Dataset[DiagnosisEvalInputs, DiagnosisEvalExecution, DiagnosisEvalMetadata]:
    """Load typed YAML cases and attach deterministic BodySense evaluators."""

    document = load_dataset_document(path)
    cases = [
        Case(name=item.name, inputs=item.inputs, metadata=item.metadata) for item in document.cases
    ]
    return Dataset(
        name=document.name,
        cases=cases,
        evaluators=[
            DiagnosisContract(),
            ExpectedStatus(),
            CandidatePolicy(),
            NoTreatmentSideEffect(),
            ConfigurationProvenance(),
            ToolTracePolicy(),
        ],
    )


def _configured_deterministic_service(
    config: DiagnosisAgentManifest,
) -> tuple[DiagnosisService, Any]:
    model = deterministic_diagnosis_model(call_tools=[])
    agent = create_diagnosis_agent(
        model,
        prompt_revision=config.prompt_revision,
        output_schema_revision=config.output_schema_revision,
        tool_policy_revision=config.tool_policy_revision,
        evidence_policy_revision=config.evidence_policy_revision,
    )
    return DiagnosisService(diagnosis_agent=agent), model


def build_deterministic_task(configuration_id: str | None = None) -> Any:
    """Instrument the real Diagnosis application path without provider nondeterminism."""

    config = (
        get_diagnosis_configuration(configuration_id)
        if configuration_id is not None
        else get_default_diagnosis_configuration()
    )

    async def task(inputs: DiagnosisEvalInputs) -> DiagnosisEvalExecution:
        service, model = _configured_deterministic_service(config)
        with capture_run_messages() as messages:
            payload = await service.generate_diagnosis(
                user_id=inputs.user_id,
                body_state_revision=inputs.body_state_revision,
                configuration_id=config.configuration_id,
                body_state=inputs.body_state,
                relevant_history=inputs.relevant_history,
                profile=inputs.profile,
            )

        parameters = getattr(model, "last_model_request_parameters", None)
        available_tools = (
            [tool.name for tool in parameters.function_tools] if parameters is not None else []
        )
        tool_calls = [
            part.tool_name
            for message in messages
            for part in message.parts
            if isinstance(part, ToolCallPart) and part.tool_name != "final_result"
        ]
        return DiagnosisEvalExecution(
            payload=payload,
            configuration_id=config.configuration_id,
            trace=DiagnosisEvalTrace(
                agent_executed=parameters is not None,
                available_tools=available_tools,
                tool_calls=tool_calls,
            ),
        )

    return task


def run_diagnosis_qualification(
    path: Path = DEFAULT_DATASET_PATH,
    *,
    configuration_id: str | None = None,
) -> DiagnosisQualificationRun:
    """Evaluate one immutable Agent configuration against the qualification dataset."""

    config = (
        get_diagnosis_configuration(configuration_id)
        if configuration_id is not None
        else get_default_diagnosis_configuration()
    )
    document = load_dataset_document(path)
    dataset = load_diagnosis_dataset(path)
    report = dataset.evaluate_sync(
        build_deterministic_task(config.configuration_id),
        progress=False,
        name="diagnosis-qualification",
    )
    return DiagnosisQualificationRun(
        report=report,
        dataset_path=path,
        dataset_fingerprint=dataset_fingerprint(document),
        configuration=config,
    )


def _bucket_increment(buckets: dict[str, dict[str, int]], name: str, passed: bool) -> None:
    bucket = buckets.setdefault(name, {"passed": 0, "total": 0})
    bucket["total"] += 1
    bucket["passed"] += int(passed)


def _rate(stats: dict[str, int]) -> float:
    total = stats.get("total", 0)
    return stats.get("passed", 0) / total if total else 0.0


def report_summary(run: DiagnosisQualificationRun) -> dict[str, Any]:
    """Convert Pydantic Evals output into stable slice-aware qualification evidence."""

    cases: list[dict[str, Any]] = []
    passed = 0
    split_totals: dict[str, dict[str, int]] = {}
    slice_totals: dict[str, dict[str, int]] = {}
    evaluator_totals: dict[str, dict[str, int]] = {}
    critical_failures: list[str] = []

    for case in run.report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        metadata = case.metadata
        split = str(getattr(metadata, "split", "unknown"))
        slices = list(getattr(metadata, "slices", ["unknown"]))
        critical = bool(getattr(metadata, "critical", False))
        _bucket_increment(split_totals, split, case_passed)
        for slice_name in slices:
            _bucket_increment(slice_totals, str(slice_name), case_passed)
        for evaluator_name, assertion_passed in assertions.items():
            _bucket_increment(evaluator_totals, evaluator_name, assertion_passed)
        if critical and not case_passed:
            critical_failures.append(str(case.name))

        execution = case.output
        trace = execution.trace if isinstance(execution, DiagnosisEvalExecution) else None
        cases.append(
            {
                "name": case.name,
                "passed": case_passed,
                "split": split,
                "slices": slices,
                "critical": critical,
                "assertions": assertions,
                "trace": trace.model_dump(mode="json") if trace is not None else {},
            }
        )

    total = len(run.report.cases)
    overall_pass_rate = passed / total if total else 0.0
    required_splits = set(DEFAULT_QUALIFICATION_POLICY["required_splits"])
    missing_splits = sorted(required_splits - set(split_totals))
    critical_slices = list(DEFAULT_QUALIFICATION_POLICY["critical_slices"])
    critical_slice_failures = [
        name
        for name in critical_slices
        if name not in slice_totals
        or _rate(slice_totals[name])
        < float(DEFAULT_QUALIFICATION_POLICY["minimum_critical_slice_pass_rate"])
    ]
    reasons: list[str] = []
    if missing_splits:
        reasons.append(f"missing required dataset splits: {', '.join(missing_splits)}")
    if overall_pass_rate < float(DEFAULT_QUALIFICATION_POLICY["minimum_overall_pass_rate"]):
        reasons.append(f"overall pass rate below qualification threshold: {overall_pass_rate:.3f}")
    if critical_failures:
        reasons.append(f"critical case failures: {', '.join(critical_failures)}")
    if critical_slice_failures:
        reasons.append(f"critical slice gate failed: {', '.join(critical_slice_failures)}")

    return {
        "name": run.report.name,
        "mode": run.mode,
        "dataset": {
            "path": str(run.dataset_path.relative_to(SERVICE_ROOT)),
            "fingerprint": run.dataset_fingerprint,
        },
        "configuration_id": run.configuration.configuration_id,
        "configuration": run.configuration.provenance(),
        "passed": passed,
        "total": total,
        "failed": total - passed,
        "pass_rate": overall_pass_rate,
        "splits": split_totals,
        "slices": slice_totals,
        "evaluators": evaluator_totals,
        "cases": cases,
        "qualification": {
            "qualified": not reasons,
            "policy": DEFAULT_QUALIFICATION_POLICY,
            "critical_failures": critical_failures,
            "reasons": reasons,
        },
    }


def compare_qualification_summaries(
    champion: dict[str, Any],
    candidate: dict[str, Any],
    *,
    margin: float | None = None,
) -> dict[str, Any]:
    """Paired non-inferiority comparison on the same immutable qualification dataset."""

    if champion.get("dataset", {}).get("fingerprint") != candidate.get("dataset", {}).get(
        "fingerprint"
    ):
        raise ValueError("qualification comparison requires the same dataset fingerprint")

    champion_cases = {str(case["name"]): case for case in champion.get("cases", [])}
    candidate_cases = {str(case["name"]): case for case in candidate.get("cases", [])}
    if champion_cases.keys() != candidate_cases.keys():
        raise ValueError("qualification comparison requires identical paired case names")

    effective_margin = (
        float(DEFAULT_QUALIFICATION_POLICY["non_inferiority_margin"]) if margin is None else margin
    )
    regressions = sorted(
        name
        for name, baseline_case in champion_cases.items()
        if baseline_case.get("passed") and not candidate_cases[name].get("passed")
    )
    improvements = sorted(
        name
        for name, baseline_case in champion_cases.items()
        if not baseline_case.get("passed") and candidate_cases[name].get("passed")
    )
    critical_regressions = [name for name in regressions if champion_cases[name].get("critical")]

    champion_rate = sum(bool(case.get("passed")) for case in champion_cases.values()) / max(
        1, len(champion_cases)
    )
    candidate_rate = sum(bool(case.get("passed")) for case in candidate_cases.values()) / max(
        1, len(candidate_cases)
    )
    pass_rate_delta = candidate_rate - champion_rate

    all_slices = set(champion.get("slices", {})) | set(candidate.get("slices", {}))
    slice_deltas: dict[str, float] = {}
    slice_regressions: list[str] = []
    for name in sorted(all_slices):
        champion_stats = champion.get("slices", {}).get(name, {"passed": 0, "total": 0})
        candidate_stats = candidate.get("slices", {}).get(name, {"passed": 0, "total": 0})
        delta = _rate(candidate_stats) - _rate(champion_stats)
        slice_deltas[name] = delta
        if delta < -effective_margin:
            slice_regressions.append(name)

    non_inferior = (
        pass_rate_delta >= -effective_margin and not critical_regressions and not slice_regressions
    )
    return {
        "champion_configuration_id": champion.get("configuration_id"),
        "candidate_configuration_id": candidate.get("configuration_id"),
        "margin": effective_margin,
        "champion_pass_rate": champion_rate,
        "candidate_pass_rate": candidate_rate,
        "pass_rate_delta": pass_rate_delta,
        "regressions": regressions,
        "improvements": improvements,
        "critical_regressions": critical_regressions,
        "slice_deltas": slice_deltas,
        "slice_regressions": slice_regressions,
        "non_inferior": non_inferior,
        "promotion_eligible": bool(candidate.get("qualification", {}).get("qualified"))
        and non_inferior,
    }


def render_summary(
    run: DiagnosisQualificationRun,
    *,
    comparison: dict[str, Any] | None = None,
) -> str:
    """Render concise human-readable qualification evidence."""

    summary = report_summary(run)
    qualification = summary["qualification"]
    lines = [
        "# Diagnosis Pydantic Evals Qualification",
        "",
        f"- Result: {summary['passed']}/{summary['total']} passed",
        f"- Qualified: {'YES' if qualification['qualified'] else 'NO'}",
        f"- Agent configuration: `{summary['configuration_id']}`",
        f"- Dataset fingerprint: `{summary['dataset']['fingerprint']}`",
        f"- Mode: {summary['mode']}",
        "",
        "## Splits",
    ]
    for name, stats in sorted(summary["splits"].items()):
        lines.append(f"- `{name}`: {stats['passed']}/{stats['total']} passed")
    lines.extend(["", "## Risk slices"])
    for name, stats in sorted(summary["slices"].items()):
        lines.append(f"- `{name}`: {stats['passed']}/{stats['total']} passed")
    lines.extend(["", "## Cases"])
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        lines.append(f"- `{mark}` `{case['name']}` [{case['split']}] ({', '.join(case['slices'])})")
    if qualification["reasons"]:
        lines.extend(["", "## Qualification failures"])
        lines.extend(f"- {reason}" for reason in qualification["reasons"])
    if comparison is not None:
        lines.extend(
            [
                "",
                "## Champion comparison",
                f"- Non-inferior: {'YES' if comparison['non_inferior'] else 'NO'}",
                f"- Promotion eligible: {'YES' if comparison['promotion_eligible'] else 'NO'}",
                f"- Pass-rate delta: {comparison['pass_rate_delta']:+.3f}",
            ]
        )
    return "\n".join(lines) + "\n"


def summary_json(
    run: DiagnosisQualificationRun,
    *,
    comparison: dict[str, Any] | None = None,
) -> str:
    summary = report_summary(run)
    if comparison is not None:
        summary["comparison"] = comparison
    return json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
