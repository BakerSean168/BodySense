"""Deterministic evaluators that grade one Diagnosis qualification execution."""

from __future__ import annotations

from dataclasses import dataclass

from pydantic_evals.evaluators import Evaluator

from src.agents.diagnosis_agent import diagnosis_tool_names
from src.evals.agent_config_archive import get_diagnosis_evaluation_configuration
from src.evals.diagnosis_qualification_schema import (
    DiagnosisEvalExecution,
    DiagnosisEvalInputs,
    DiagnosisEvalMetadata,
    EvalContext,
)
from src.evals.retired_diagnosis_runtime import (
    RETIRED_DIAGNOSIS_TOOL_POLICY_V1,
    retired_diagnosis_tool_names,
)


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
        config = get_diagnosis_evaluation_configuration(ctx.output.configuration_id)
        if not metadata.expected_agent_executed:
            expected_tools = []
        elif config.tool_policy_revision == RETIRED_DIAGNOSIS_TOOL_POLICY_V1:
            expected_tools = retired_diagnosis_tool_names(config.tool_policy_revision)
        else:
            expected_tools = diagnosis_tool_names(config.tool_policy_revision)
        if sorted(trace.available_tools) != sorted(expected_tools):
            return False
        if len(trace.tool_calls) > metadata.max_tool_calls:
            return False
        return all(name in expected_tools for name in trace.tool_calls)
