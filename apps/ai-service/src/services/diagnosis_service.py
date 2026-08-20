"""Typed BodyState Diagnosis application orchestration.

Diagnosis has one execution path: an exact durable BodyState revision is supplied
as run input, the PydanticAI Agent returns typed output, and deterministic safety /
governance wraps that output before Go persists an immutable DiagnosisAnalysis.
There is deliberately no extracted-info fallback and no Treatment generation here.
"""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from pydantic_ai.models import Model

from ..agents.diagnosis_agent import (
    DIAGNOSIS_EVIDENCE_POLICY_REVISION,
    create_diagnosis_agent,
)
from ..agents.evidence import (
    DIAGNOSIS_EVIDENCE_POLICY_V2,
    DiagnosisEvidenceAcquirer,
    KnowledgeEvidenceSearcher,
)
from ..ai.diagnosis_gateway_model import diagnosis_model_settings, get_diagnosis_runtime_model
from ..configuration.diagnosis_agent_config import (
    DiagnosisAgentManifest,
    get_diagnosis_configuration,
)
from ..models.dependencies import EvidenceSearcher
from ..models.diagnosis import DiagnosisAgentOutput, DiagnosisDependencies
from ..models.evidence import EvidenceAcquisitionTrace, EvidenceBudget
from ..runtime.governance import guard_structured_output
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_diagnosis_model,
)
from .red_flag_detector import get_red_flag_detector

ModelResolver = Callable[[DiagnosisAgentManifest], Model]
EvidenceSearcherFactory = Callable[[str], EvidenceSearcher]


class DiagnosisService:
    def __init__(
        self,
        *,
        model_resolver: ModelResolver | None = None,
        evidence_searcher_factory: EvidenceSearcherFactory | None = None,
    ) -> None:
        self._model_resolver = model_resolver or get_diagnosis_runtime_model
        self._evidence_searcher_factory = evidence_searcher_factory

    async def generate_diagnosis(
        self,
        *,
        user_id: str = "",
        body_state_revision: int,
        configuration_id: str,
        body_state: dict[str, Any],
        relevant_history: list[dict[str, Any]] | None = None,
        profile: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Generate a governed analysis from one pinned durable BodyState revision."""

        if body_state_revision <= 0:
            raise ValueError("body_state_revision must reference a durable revision")
        if not body_state:
            raise ValueError("body_state is required for diagnosis")
        current_revision = int(body_state.get("current_revision") or 0)
        if current_revision and current_revision != body_state_revision:
            raise ValueError("body_state_revision does not match body_state.current_revision")

        config = get_diagnosis_configuration(configuration_id)
        profile = profile or {}
        relevant_history = relevant_history or []
        red_flag_input = _body_state_to_extracted_info(body_state)
        detector = get_red_flag_detector()
        red_flag_result = detector.detect(
            red_flag_input,
            _body_state_safety_text(body_state),
        )
        if red_flag_result.has_red_flags:
            blocked: dict[str, Any] = {
                "status": "safety_blocked",
                "scope": "full_body",
                "summary": "当前信息包含需要优先处理的安全信号，暂不生成普通可能性候选。",
                "candidates": [],
                "cross_concern_patterns": [],
                "information_gaps": [],
                "safety_summary": {"red_flags": red_flag_result.to_dict()},
                "red_flags": red_flag_result.to_dict(),
            }
            blocked["agent_configuration"] = config.provenance()
            guarded = guard_structured_output(
                "diagnosis",
                blocked,
                rag_results=[],
                extracted_info=red_flag_input,
                policy_revision=config.governance_policy_revision,
            )
            return _emit_with_configuration(
                guarded.to_emit_dict(),
                config,
                execution_provenance=_bypassed_execution_provenance(
                    config, "python_pre_agent_safety_gate"
                ),
            )

        output, citations, evidence_trace, execution_provenance = await self._run_typed_agent(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            relevant_history=relevant_history,
            profile=profile,
            config=config,
        )

        validated: dict[str, Any] = {
            "status": output.status,
            "scope": output.scope,
            "summary": output.summary,
            "candidates": [
                candidate.model_dump(mode="json", exclude_none=True)
                for candidate in output.candidates
            ],
            "cross_concern_patterns": output.cross_concern_patterns,
            "information_gaps": _merge_information_gaps(output.information_gaps, evidence_trace),
            "safety_summary": output.safety_summary,
            "agent_configuration": config.provenance(),
        }
        if citations:
            validated["citations"] = citations
        if evidence_trace is not None:
            validated["evidence_acquisition"] = evidence_trace.model_dump(mode="json")
        guarded = guard_structured_output(
            "diagnosis",
            validated,
            rag_results=citations,
            extracted_info=red_flag_input,
            policy_revision=config.governance_policy_revision,
        )
        return _emit_with_configuration(
            guarded.to_emit_dict(), config, execution_provenance=execution_provenance
        )

    async def _run_typed_agent(
        self,
        *,
        user_id: str,
        body_state_revision: int,
        body_state: dict[str, Any],
        relevant_history: list[dict[str, Any]],
        profile: dict[str, Any],
        config: DiagnosisAgentManifest,
    ) -> tuple[
        DiagnosisAgentOutput,
        list[dict[str, Any]],
        EvidenceAcquisitionTrace | None,
        dict[str, Any],
    ]:
        searcher: EvidenceSearcher | None = None
        if user_id and self._evidence_searcher_factory is not None:
            searcher = self._evidence_searcher_factory(user_id)

        evidence_acquirer: DiagnosisEvidenceAcquirer | None = None
        if config.evidence_policy_revision == DIAGNOSIS_EVIDENCE_POLICY_V2:
            evidence_acquirer = DiagnosisEvidenceAcquirer(
                searcher=searcher,
                budget=EvidenceBudget(max_searches=2, max_results_per_search=5),
                policy_revision=config.evidence_policy_revision,
            )
        elif config.evidence_policy_revision != DIAGNOSIS_EVIDENCE_POLICY_REVISION:
            raise ValueError(
                f"unsupported Diagnosis evidence policy revision: {config.evidence_policy_revision}"
            )

        deps = DiagnosisDependencies(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            relevant_history=relevant_history,
            profile=profile,
            evidence_searcher=searcher,
            evidence_acquirer=evidence_acquirer,
        )
        run_kwargs: dict[str, Any] = {
            "deps": deps,
            "model": self._model_resolver(config),
            "model_settings": diagnosis_model_settings(config),
        }
        agent = create_diagnosis_agent(
            prompt_revision=config.prompt_revision,
            output_schema_revision=config.output_schema_revision,
            tool_policy_revision=config.tool_policy_revision,
            evidence_policy_revision=config.evidence_policy_revision,
        )
        result = await agent.run(
            "Synthesize all supported possible-diagnosis candidates from the pinned durable state.",
            **run_kwargs,
        )
        evidence_trace = evidence_acquirer.trace() if evidence_acquirer is not None else None
        return (
            result.output,
            _dedupe_evidence(deps.retrieved_evidence),
            evidence_trace,
            _execution_provenance(result, config),
        )


def _merge_information_gaps(
    model_gaps: list[str], evidence_trace: EvidenceAcquisitionTrace | None
) -> list[str]:
    """Preserve critical unresolved gaps even when the model omits them after tool use."""

    merged = list(model_gaps)
    if evidence_trace is not None:
        merged.extend(gap.description for gap in evidence_trace.unresolved_critical_gaps)
    return list(dict.fromkeys(gap.strip() for gap in merged if gap.strip()))


def _emit_with_configuration(
    payload: dict[str, Any],
    config: DiagnosisAgentManifest,
    *,
    execution_provenance: dict[str, Any],
) -> dict[str, Any]:
    """Attach immutable configuration and runtime provenance after governance filtering."""
    result = dict(payload)
    result["agent_configuration"] = config.provenance()
    result["execution_provenance"] = execution_provenance
    return result


def _execution_provenance(result: Any, config: DiagnosisAgentManifest) -> dict[str, Any]:
    response = result.response
    usage = result.usage
    return {
        "status": "executed",
        "runtime": "pydantic-ai",
        "logical_model": config.logical_model,
        "model_group_revision": config.model_group_revision,
        "gateway_reported_model": response.model_name,
        "provider_adapter": response.provider_name,
        "agent_run_id": str(response.run_id) if response.run_id is not None else None,
        "conversation_id": (
            str(response.conversation_id) if response.conversation_id is not None else None
        ),
        "usage": {
            "requests": usage.requests,
            "input_tokens": usage.input_tokens,
            "output_tokens": usage.output_tokens,
            "total_tokens": (usage.input_tokens or 0) + (usage.output_tokens or 0),
        },
    }


def _bypassed_execution_provenance(config: DiagnosisAgentManifest, reason: str) -> dict[str, Any]:
    return {
        "status": "bypassed",
        "runtime": "pydantic-ai",
        "reason": reason,
        "logical_model": config.logical_model,
        "model_group_revision": config.model_group_revision,
    }


def _dedupe_evidence(items: list[dict[str, Any]]) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    seen: set[str] = set()
    for item in items:
        identity = str(item.get("evidence_id") or item.get("source_key") or item.get("id") or item)
        if identity in seen:
            continue
        seen.add(identity)
        result.append(item)
    return result


def _body_state_safety_text(body_state: dict[str, Any]) -> str:
    parts: list[str] = []
    for fact in body_state.get("facts", []) or []:
        if not isinstance(fact, dict):
            continue
        if fact.get("kind") not in {"discomfort", "red_flags", "safety_finding"}:
            continue
        value = str(fact.get("value") or "").strip()
        if value:
            parts.append(value)
        raw_details = fact.get("details")
        details: dict[str, Any] = raw_details if isinstance(raw_details, dict) else {}
        notes = str(details.get("additional_notes") or "").strip()
        if notes:
            parts.append(notes)
    return " ".join(parts)


def _body_state_to_extracted_info(body_state: dict[str, Any]) -> list[dict[str, Any]]:
    """Adapt durable current facts only for the deterministic red-flag detector."""

    items: list[dict[str, Any]] = []
    for fact in body_state.get("facts", []) or []:
        if not isinstance(fact, dict):
            continue
        if fact.get("kind") not in {"discomfort", "red_flags", "safety_finding"}:
            continue
        raw_details = fact.get("details")
        details: dict[str, Any] = raw_details if isinstance(raw_details, dict) else {}
        items.append(
            {
                "body_part": fact.get("body_region", ""),
                "symptom_type": fact.get("value", ""),
                "duration": details.get("duration", ""),
                "trigger": details.get("trigger", ""),
                "relief": details.get("relief", ""),
                "severity": details.get("severity", ""),
            }
        )
    return items


_diagnosis_service: DiagnosisService | None = None


def get_diagnosis_service() -> DiagnosisService:
    global _diagnosis_service
    if _diagnosis_service is None:
        if deterministic_ai_enabled():
            _diagnosis_service = DiagnosisService(
                model_resolver=lambda _config: deterministic_diagnosis_model()
            )
        else:
            _diagnosis_service = DiagnosisService(
                model_resolver=get_diagnosis_runtime_model,
                evidence_searcher_factory=KnowledgeEvidenceSearcher,
            )
    return _diagnosis_service
