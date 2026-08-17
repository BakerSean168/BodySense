"""Application service for typed Treatment Agent execution."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from pydantic_ai import Agent
from pydantic_ai.models import Model

from ..agents.evidence import KnowledgeEvidenceSearcher
from ..agents.treatment_agent import create_treatment_agent
from ..ai.pydantic_model import get_pydantic_model, route_model_settings
from ..models.dependencies import EvidenceSearcher
from ..models.treatment import TreatmentAgentOutput, TreatmentDependencies
from ..runtime.governance import guard_structured_output
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_treatment_model,
)

ModelResolver = Callable[[str], Model]
EvidenceSearcherFactory = Callable[[str], EvidenceSearcher]


class TreatmentAgentService:
    def __init__(
        self,
        *,
        proposal_agent: Agent[TreatmentDependencies, TreatmentAgentOutput] | None = None,
        model_resolver: ModelResolver | None = None,
        evidence_searcher_factory: EvidenceSearcherFactory | None = None,
    ) -> None:
        self._proposal_agent = proposal_agent or create_treatment_agent()
        self._model_resolver = model_resolver
        self._evidence_searcher_factory = evidence_searcher_factory

    async def recommend(
        self,
        *,
        user_id: str,
        body_state_revision: int,
        body_state: dict[str, Any],
        diagnosis_analysis: dict[str, Any],
        candidate_assessments: list[dict[str, Any]],
        profile: dict[str, Any],
        user_constraints: dict[str, Any],
        evidence: list[dict[str, Any]],
        use_case: str,
    ) -> dict[str, Any]:
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
        kwargs: dict[str, Any] = {"deps": deps}
        if self._model_resolver is not None:
            kwargs["model"] = self._model_resolver(use_case)
            kwargs["model_settings"] = route_model_settings(use_case)
        result = await self._proposal_agent.run(
            "Create one reviewable intervention proposal from the pinned durable inputs.",
            **kwargs,
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
        return guard_structured_output("treatment", payload).to_emit_dict()


_treatment_agent_service: TreatmentAgentService | None = None


def get_treatment_agent_service() -> TreatmentAgentService:
    global _treatment_agent_service
    if _treatment_agent_service is None:
        if deterministic_ai_enabled():
            _treatment_agent_service = TreatmentAgentService(
                proposal_agent=create_treatment_agent(deterministic_treatment_model())
            )
        else:
            _treatment_agent_service = TreatmentAgentService(
                model_resolver=get_pydantic_model,
                evidence_searcher_factory=KnowledgeEvidenceSearcher,
            )
    return _treatment_agent_service
