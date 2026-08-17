"""Typed BodyState Diagnosis application orchestration.

Diagnosis has one execution path: an exact durable BodyState revision is supplied
as run input, the PydanticAI Agent returns typed output, and deterministic safety /
governance wraps that output before Go persists an immutable DiagnosisAnalysis.
There is deliberately no extracted-info fallback and no Treatment generation here.
"""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from pydantic_ai import Agent
from pydantic_ai.models import Model

from ..agents.diagnosis_agent import create_diagnosis_agent
from ..agents.evidence import KnowledgeEvidenceSearcher
from ..ai.pydantic_model import get_pydantic_model, route_model_settings
from ..models.dependencies import EvidenceSearcher
from ..models.diagnosis import DiagnosisAgentOutput, DiagnosisDependencies
from ..runtime.governance import guard_structured_output
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_diagnosis_model,
)
from .red_flag_detector import get_red_flag_detector

ModelResolver = Callable[[str], Model]
EvidenceSearcherFactory = Callable[[str], EvidenceSearcher]


class DiagnosisService:
    def __init__(
        self,
        *,
        diagnosis_agent: Agent[DiagnosisDependencies, DiagnosisAgentOutput] | None = None,
        model_resolver: ModelResolver | None = None,
        evidence_searcher_factory: EvidenceSearcherFactory | None = None,
    ) -> None:
        self._agent = diagnosis_agent or create_diagnosis_agent()
        self._model_resolver = (
            model_resolver
            if model_resolver is not None
            else (None if diagnosis_agent is not None else get_pydantic_model)
        )
        self._evidence_searcher_factory = evidence_searcher_factory

    async def generate_diagnosis(
        self,
        *,
        user_id: str = "",
        body_state_revision: int,
        body_state: dict[str, Any],
        relevant_history: list[dict[str, Any]] | None = None,
        profile: dict[str, Any] | None = None,
        rag_context: str = "",
        rag_results: list[dict[str, Any]] | None = None,
        use_case: str = "llm.json",
    ) -> dict[str, Any]:
        """Generate a governed analysis from one pinned durable BodyState revision."""

        if body_state_revision <= 0:
            raise ValueError("body_state_revision must reference a durable revision")
        if not body_state:
            raise ValueError("body_state is required for diagnosis")
        current_revision = int(body_state.get("current_revision") or 0)
        if current_revision and current_revision != body_state_revision:
            raise ValueError("body_state_revision does not match body_state.current_revision")

        profile = profile or {}
        relevant_history = relevant_history or []
        rag_results = rag_results or []

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
            if rag_results:
                blocked["citations"] = rag_results
            return guard_structured_output(
                "diagnosis",
                blocked,
                rag_results=rag_results,
                extracted_info=red_flag_input,
            ).to_emit_dict()

        output, citations = await self._run_typed_agent(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            relevant_history=relevant_history,
            profile=profile,
            rag_context=rag_context,
            rag_results=rag_results,
            use_case=use_case,
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
            "information_gaps": output.information_gaps,
            "safety_summary": output.safety_summary,
        }
        if citations:
            validated["citations"] = citations
        return guard_structured_output(
            "diagnosis",
            validated,
            rag_results=citations or rag_results,
            extracted_info=red_flag_input,
        ).to_emit_dict()

    async def _run_typed_agent(
        self,
        *,
        user_id: str,
        body_state_revision: int,
        body_state: dict[str, Any],
        relevant_history: list[dict[str, Any]],
        profile: dict[str, Any],
        rag_context: str,
        rag_results: list[dict[str, Any]],
        use_case: str,
    ) -> tuple[DiagnosisAgentOutput, list[dict[str, Any]]]:
        searcher: EvidenceSearcher | None = None
        if user_id and self._evidence_searcher_factory is not None:
            searcher = self._evidence_searcher_factory(user_id)
        deps = DiagnosisDependencies(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            relevant_history=relevant_history,
            profile=profile,
            rag_context=rag_context,
            evidence_searcher=searcher,
            retrieved_evidence=list(rag_results),
        )
        run_kwargs: dict[str, Any] = {"deps": deps}
        if self._model_resolver is not None:
            run_kwargs["model"] = self._model_resolver(use_case)
            run_kwargs["model_settings"] = route_model_settings(use_case)
        result = await self._agent.run(
            "Synthesize all supported possible-diagnosis candidates from the pinned durable state.",
            **run_kwargs,
        )
        return result.output, _dedupe_evidence(deps.retrieved_evidence)


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
                diagnosis_agent=create_diagnosis_agent(deterministic_diagnosis_model())
            )
        else:
            _diagnosis_service = DiagnosisService(
                model_resolver=get_pydantic_model,
                evidence_searcher_factory=KnowledgeEvidenceSearcher,
            )
    return _diagnosis_service
