"""Application service for immutable-config-aware Treatment Agent execution."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from pydantic_ai.models import Model

from ..agents.evidence import KnowledgeEvidenceSearcher
from ..agents.treatment_agent import create_treatment_agent
from ..ai.treatment_gateway_model import get_treatment_runtime_model, treatment_model_settings
from ..configuration.treatment_agent_config import (
    TreatmentAgentManifest,
    get_treatment_configuration,
)
from ..models.dependencies import EvidenceSearcher
from ..models.treatment import TreatmentDependencies
from ..runtime.governance import guard_structured_output
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_treatment_model,
)

ModelResolver = Callable[[TreatmentAgentManifest], Model]
EvidenceSearcherFactory = Callable[[str], EvidenceSearcher]


class TreatmentAgentService:
    def __init__(
        self,
        *,
        model_resolver: ModelResolver | None = None,
        evidence_searcher_factory: EvidenceSearcherFactory | None = None,
    ) -> None:
        self._model_resolver = model_resolver or get_treatment_runtime_model
        self._evidence_searcher_factory = evidence_searcher_factory

    async def recommend(
        self,
        *,
        user_id: str,
        body_state_revision: int,
        configuration_id: str,
        body_state: dict[str, Any],
        diagnosis_analysis: dict[str, Any],
        candidate_assessments: list[dict[str, Any]],
        profile: dict[str, Any],
        user_constraints: dict[str, Any],
        evidence: list[dict[str, Any]],
    ) -> dict[str, Any]:
        if body_state_revision <= 0:
            raise ValueError("body_state_revision must reference a durable revision")
        config = get_treatment_configuration(configuration_id)
        searcher = (
            self._evidence_searcher_factory(user_id)
            if user_id and self._evidence_searcher_factory is not None
            else None
        )
        deps = TreatmentDependencies(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            diagnosis_analysis=diagnosis_analysis,
            candidate_assessments=candidate_assessments,
            profile=profile,
            user_constraints=user_constraints,
            evidence=evidence,
            evidence_searcher=searcher,
            retrieved_evidence=list(evidence),
        )
        agent = create_treatment_agent(
            prompt_revision=config.prompt_revision,
            output_schema_revision=config.output_schema_revision,
            tool_policy_revision=config.tool_policy_revision,
            evidence_policy_revision=config.evidence_policy_revision,
        )
        run_kwargs: dict[str, Any] = {
            "deps": deps,
            "model": self._model_resolver(config),
            "model_settings": treatment_model_settings(config),
        }
        result = await agent.run(
            "Create one reviewable intervention proposal from the pinned durable inputs.",
            **run_kwargs,
        )
        payload = result.output.model_dump(mode="json")
        valid_ids = {
            str(item.get("evidence_id", ""))
            for item in deps.retrieved_evidence
            if item.get("evidence_id")
        }
        payload["evidence_ids"] = [
            evidence_id
            for evidence_id in payload.get("evidence_ids", [])
            if evidence_id in valid_ids
        ]
        payload["citations"] = deps.retrieved_evidence
        guarded = guard_structured_output(
            "treatment",
            payload,
            rag_results=deps.retrieved_evidence,
            policy_revision=config.governance_policy_revision,
        )
        emitted = guarded.to_emit_dict()
        emitted["agent_configuration"] = config.provenance()
        emitted["execution_provenance"] = _execution_provenance(result, config)
        return emitted


def _execution_provenance(result: Any, config: TreatmentAgentManifest) -> dict[str, Any]:
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


_treatment_agent_service: TreatmentAgentService | None = None


def get_treatment_agent_service() -> TreatmentAgentService:
    global _treatment_agent_service
    if _treatment_agent_service is None:
        if deterministic_ai_enabled():
            _treatment_agent_service = TreatmentAgentService(
                model_resolver=lambda _config: deterministic_treatment_model()
            )
        else:
            _treatment_agent_service = TreatmentAgentService(
                model_resolver=get_treatment_runtime_model,
                evidence_searcher_factory=KnowledgeEvidenceSearcher,
            )
    return _treatment_agent_service
