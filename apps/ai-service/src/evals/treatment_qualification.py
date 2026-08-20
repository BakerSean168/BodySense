"""Deterministic Pydantic Evals baseline for immutable Treatment Agent configs."""

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

from src.agents.treatment_agent import treatment_tool_names
from src.configuration.treatment_agent_config import (
    TreatmentAgentManifest,
    get_default_treatment_configuration,
    get_treatment_configuration,
)
from src.services.treatment_agent_service import TreatmentAgentService
from src.testing_support.deterministic_ai import deterministic_treatment_model

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_DATASET_PATH = SERVICE_ROOT / "data" / "evals" / "treatment_qualification.yaml"
DEFAULT_REPORT_PATH = (
    SERVICE_ROOT / "data" / "evals" / "reports" / "treatment_champion_baseline.json"
)

EvalSplit = Literal["development", "holdout", "regression", "challenge"]


class TreatmentEvalInputs(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    user_id: str = "eval-treatment-user"
    body_state_revision: int = Field(gt=0)
    body_state: dict[str, Any]
    diagnosis_analysis: dict[str, Any]
    candidate_assessments: list[dict[str, Any]] = Field(min_length=1)
    profile: dict[str, Any] = Field(default_factory=dict)
    user_constraints: dict[str, Any] = Field(default_factory=dict)
    evidence: list[dict[str, Any]] = Field(default_factory=list)


class TreatmentEvalMetadata(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    split: EvalSplit
    slices: list[str] = Field(min_length=1)
    required_assessment_states: list[str] = Field(default_factory=list)
    required_context_tokens: list[str] = Field(default_factory=list)


class TreatmentEvalCaseDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    inputs: TreatmentEvalInputs
    metadata: TreatmentEvalMetadata


class TreatmentDatasetDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    name: str = Field(min_length=1)
    cases: list[TreatmentEvalCaseDocument] = Field(min_length=1)


class TreatmentEvalTrace(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    available_tools: list[str] = Field(default_factory=list)
    tool_calls: list[str] = Field(default_factory=list)
    run_messages: str


class TreatmentEvalExecution(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    payload: dict[str, Any]
    configuration_id: str
    trace: TreatmentEvalTrace


@dataclass(frozen=True)
class TreatmentQualificationRun:
    report: Any
    dataset_path: Path
    dataset_fingerprint: str
    configuration: TreatmentAgentManifest


EvalContext = EvaluatorContext[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]


@dataclass
class TreatmentContract(
    Evaluator[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]
):
    def evaluate(self, ctx: EvalContext) -> bool:
        payload = ctx.output.payload
        return (
            payload.get("status") == "proposed"
            and isinstance(payload.get("goal"), str)
            and isinstance(payload.get("interventions"), list)
            and bool(payload.get("interventions"))
            and isinstance(payload.get("governance"), dict)
            and payload["governance"].get("kind") == "treatment"
            and payload["governance"].get("verdict") in {"accepted", "degraded"}
        )


@dataclass
class ProposalOnly(Evaluator[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]):
    def evaluate(self, ctx: EvalContext) -> bool:
        forbidden = {
            "treatment_id",
            "treatment_revision_id",
            "revision_id",
            "accepted_at",
            "current_treatment",
            "training_plan",
        }
        return not any(field in ctx.output.payload for field in forbidden)


@dataclass
class ConfigurationProvenance(
    Evaluator[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]
):
    def evaluate(self, ctx: EvalContext) -> bool:
        config = ctx.output.payload.get("agent_configuration")
        execution = ctx.output.payload.get("execution_provenance")
        return (
            isinstance(config, dict)
            and config.get("id") == ctx.output.configuration_id
            and config.get("role") == "treatment"
            and isinstance(execution, dict)
            and execution.get("status") == "executed"
            and execution.get("runtime") == "pydantic-ai"
        )


@dataclass
class PinnedContext(Evaluator[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]):
    def evaluate(self, ctx: EvalContext) -> bool:
        metadata = ctx.metadata
        if metadata is None:
            return False
        trace = ctx.output.trace.run_messages
        if f"R{ctx.inputs.body_state_revision}" not in trace:
            return False
        states = {str(item.get("state", "")) for item in ctx.inputs.candidate_assessments}
        if not all(
            state in states and state in trace for state in metadata.required_assessment_states
        ):
            return False
        return all(token in trace for token in metadata.required_context_tokens)


@dataclass
class ToolTrace(Evaluator[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]):
    def evaluate(self, ctx: EvalContext) -> bool:
        config = get_treatment_configuration(ctx.output.configuration_id)
        expected = treatment_tool_names(config.tool_policy_revision)
        return (
            sorted(ctx.output.trace.available_tools) == sorted(expected)
            and not ctx.output.trace.tool_calls
        )


def load_dataset_document(path: Path = DEFAULT_DATASET_PATH) -> TreatmentDatasetDocument:
    return TreatmentDatasetDocument.model_validate(yaml.safe_load(path.read_text(encoding="utf-8")))


def dataset_fingerprint(document: TreatmentDatasetDocument) -> str:
    canonical = json.dumps(
        document.model_dump(mode="json"),
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    )
    return hashlib.sha256(canonical.encode()).hexdigest()


def load_treatment_dataset(
    path: Path = DEFAULT_DATASET_PATH,
) -> Dataset[TreatmentEvalInputs, TreatmentEvalExecution, TreatmentEvalMetadata]:
    document = load_dataset_document(path)
    return Dataset(
        name=document.name,
        cases=[
            Case(name=item.name, inputs=item.inputs, metadata=item.metadata)
            for item in document.cases
        ],
        evaluators=[
            TreatmentContract(),
            ProposalOnly(),
            ConfigurationProvenance(),
            PinnedContext(),
            ToolTrace(),
        ],
    )


def build_deterministic_task(configuration_id: str | None = None) -> Any:
    config = (
        get_treatment_configuration(configuration_id)
        if configuration_id is not None
        else get_default_treatment_configuration()
    )

    async def task(inputs: TreatmentEvalInputs) -> TreatmentEvalExecution:
        model = deterministic_treatment_model(call_tools=[])
        service = TreatmentAgentService(model_resolver=lambda _config: model)
        with capture_run_messages() as messages:
            payload = await service.recommend(
                user_id=inputs.user_id,
                body_state_revision=inputs.body_state_revision,
                configuration_id=config.configuration_id,
                body_state=inputs.body_state,
                diagnosis_analysis=inputs.diagnosis_analysis,
                candidate_assessments=inputs.candidate_assessments,
                profile=inputs.profile,
                user_constraints=inputs.user_constraints,
                evidence=inputs.evidence,
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
        return TreatmentEvalExecution(
            payload=payload,
            configuration_id=config.configuration_id,
            trace=TreatmentEvalTrace(
                available_tools=available_tools,
                tool_calls=tool_calls,
                run_messages=str(messages),
            ),
        )

    return task


def run_treatment_qualification(
    path: Path = DEFAULT_DATASET_PATH,
    *,
    configuration_id: str | None = None,
) -> TreatmentQualificationRun:
    config = (
        get_treatment_configuration(configuration_id)
        if configuration_id is not None
        else get_default_treatment_configuration()
    )
    document = load_dataset_document(path)
    report = load_treatment_dataset(path).evaluate_sync(
        build_deterministic_task(config.configuration_id),
        progress=False,
        name="treatment-qualification",
    )
    return TreatmentQualificationRun(
        report=report,
        dataset_path=path,
        dataset_fingerprint=dataset_fingerprint(document),
        configuration=config,
    )


def report_summary(run: TreatmentQualificationRun) -> dict[str, Any]:
    cases: list[dict[str, Any]] = []
    passed = 0
    splits: dict[str, dict[str, int]] = {}
    slices: dict[str, dict[str, int]] = {}
    for case in run.report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        metadata = case.metadata
        split = str(getattr(metadata, "split", "unknown"))
        case_slices = list(getattr(metadata, "slices", []))
        _increment(splits, split, case_passed)
        for slice_name in case_slices:
            _increment(slices, str(slice_name), case_passed)
        cases.append(
            {
                "name": case.name,
                "passed": case_passed,
                "split": split,
                "slices": case_slices,
                "assertions": assertions,
            }
        )
    total = len(cases)
    qualified = (
        total > 0
        and passed == total
        and set(splits)
        == {
            "development",
            "holdout",
            "regression",
            "challenge",
        }
    )
    return {
        "name": run.report.name,
        "mode": "deterministic",
        "dataset": {
            "path": str(run.dataset_path.relative_to(SERVICE_ROOT)),
            "fingerprint": run.dataset_fingerprint,
        },
        "configuration_id": run.configuration.configuration_id,
        "configuration": run.configuration.provenance(),
        "passed": passed,
        "total": total,
        "failed": total - passed,
        "splits": splits,
        "slices": slices,
        "cases": cases,
        "qualification": {
            "qualified": qualified,
            "reasons": []
            if qualified
            else ["all four splits and all deterministic evaluators must pass"],
        },
    }


def _increment(buckets: dict[str, dict[str, int]], name: str, passed: bool) -> None:
    bucket = buckets.setdefault(name, {"passed": 0, "total": 0})
    bucket["total"] += 1
    bucket["passed"] += int(passed)


def render_summary(run: TreatmentQualificationRun) -> str:
    summary = report_summary(run)
    lines = [
        "# Treatment Pydantic Evals Qualification",
        "",
        f"- Result: {summary['passed']}/{summary['total']} passed",
        f"- Qualified: {'YES' if summary['qualification']['qualified'] else 'NO'}",
        f"- Agent configuration: `{summary['configuration_id']}`",
        f"- Dataset fingerprint: `{summary['dataset']['fingerprint']}`",
        "",
        "## Cases",
    ]
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        lines.append(f"- `{mark}` `{case['name']}` [{case['split']}] ({', '.join(case['slices'])})")
    return "\n".join(lines) + "\n"


def summary_json(run: TreatmentQualificationRun) -> str:
    return json.dumps(report_summary(run), ensure_ascii=False, indent=2, sort_keys=True) + "\n"


def compare_treatment_qualification_summaries(
    champion: dict[str, Any],
    challenger: dict[str, Any],
    *,
    non_inferiority_margin: float = 0.0,
) -> dict[str, Any]:
    """Paired deterministic comparison on the exact same Treatment dataset."""

    champion_dataset = champion.get("dataset", {}).get("fingerprint")
    challenger_dataset = challenger.get("dataset", {}).get("fingerprint")
    if not champion_dataset or champion_dataset != challenger_dataset:
        raise ValueError("Treatment qualification comparison requires one dataset fingerprint")

    champion_cases = {str(case["name"]): bool(case["passed"]) for case in champion["cases"]}
    challenger_cases = {str(case["name"]): bool(case["passed"]) for case in challenger["cases"]}
    if set(champion_cases) != set(challenger_cases):
        raise ValueError("Treatment qualification comparison requires the same case identities")

    regressions = [
        name for name, passed in champion_cases.items() if passed and not challenger_cases[name]
    ]
    champion_rate = float(champion["passed"]) / max(1, int(champion["total"]))
    challenger_rate = float(challenger["passed"]) / max(1, int(challenger["total"]))
    delta = challenger_rate - champion_rate
    non_inferior = delta >= -non_inferiority_margin and not regressions
    return {
        "dataset_fingerprint": champion_dataset,
        "champion_configuration_id": champion["configuration_id"],
        "challenger_configuration_id": challenger["configuration_id"],
        "champion_pass_rate": champion_rate,
        "challenger_pass_rate": challenger_rate,
        "pass_rate_delta": delta,
        "non_inferiority_margin": non_inferiority_margin,
        "regressions": regressions,
        "non_inferior": non_inferior,
        "promotion_eligible": bool(
            non_inferior
            and champion.get("qualification", {}).get("qualified")
            and challenger.get("qualification", {}).get("qualified")
        ),
    }
