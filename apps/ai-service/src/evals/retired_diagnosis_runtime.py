"""Offline-only execution support for retired Diagnosis manifests.

Nothing in this module is imported by the production Diagnosis runtime. It exists
only so historical qualification evidence can be reproduced after retired tool
and evidence policies are removed from ``src.agents.diagnosis_agent``.
"""

from __future__ import annotations

import json
from typing import Any

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from src.configuration.diagnosis_agent_config import DiagnosisAgentManifest
from src.models.dependencies import EvidenceSearcher
from src.models.diagnosis import (
    DiagnosisAgentOutput,
    DiagnosisDependencies,
    get_diagnosis_output_type,
)
from src.prompts.diagnosis import get_diagnosis_system_prompt
from src.services.diagnosis_service import (
    DiagnosisService,
    _dedupe_evidence,
    _execution_provenance,
)

RETIRED_DIAGNOSIS_TOOL_POLICY_V1 = "diagnosis-tools-legacy-v1"
RETIRED_DIAGNOSIS_EVIDENCE_POLICY_V1 = "diagnosis-evidence-legacy-v1"


def retired_diagnosis_tool_names(tool_policy_revision: str) -> list[str]:
    if tool_policy_revision != RETIRED_DIAGNOSIS_TOOL_POLICY_V1:
        raise ValueError(
            f"unsupported retired Diagnosis tool policy revision: {tool_policy_revision}"
        )
    return ["search_evidence"]


def _create_retired_v1_agent(
    model: Model | str | None = None,
    *,
    config: DiagnosisAgentManifest,
) -> Agent[DiagnosisDependencies, DiagnosisAgentOutput]:
    if config.tool_policy_revision != RETIRED_DIAGNOSIS_TOOL_POLICY_V1:
        raise ValueError(
            f"unsupported retired Diagnosis tool policy revision: {config.tool_policy_revision}"
        )
    if config.evidence_policy_revision != RETIRED_DIAGNOSIS_EVIDENCE_POLICY_V1:
        raise ValueError(
            "unsupported retired Diagnosis evidence policy revision: "
            f"{config.evidence_policy_revision}"
        )

    agent = Agent(
        model,
        deps_type=DiagnosisDependencies,
        output_type=get_diagnosis_output_type(config.output_schema_revision),
        system_prompt=get_diagnosis_system_prompt(config.prompt_revision),
        name="bodysense_diagnosis_offline_historical",
        retries=2,
    )

    @agent.instructions
    def body_state_context(ctx: RunContext[DiagnosisDependencies]) -> str:
        deps = ctx.deps
        return (
            "Analyze the exact durable BodyState revision supplied for this run.\n"
            f"BodyState revision: R{deps.body_state_revision}\n"
            f"BodyState JSON: {json.dumps(deps.body_state, ensure_ascii=False)}\n"
            f"Relevant history JSON: {json.dumps(deps.relevant_history, ensure_ascii=False)}\n"
            f"Profile JSON: {json.dumps(deps.profile, ensure_ascii=False)}\n"
            "Use search_evidence only for a concrete evidence gap that materially "
            "affects a candidate. Do not search merely to decorate the response."
        )

    @agent.tool
    async def search_evidence(
        ctx: RunContext[DiagnosisDependencies],
        query: str,
        top_k: int = 5,
    ) -> list[dict[str, object]]:
        searcher = ctx.deps.evidence_searcher
        if searcher is None:
            return []
        outcome = await searcher.search(query, top_k=top_k)
        results = [dict(item) for item in outcome.evidence]
        known = {str(item.get("evidence_id", "")) for item in ctx.deps.retrieved_evidence}
        for item in results:
            evidence_id = str(item.get("evidence_id", ""))
            if evidence_id and evidence_id not in known:
                ctx.deps.retrieved_evidence.append(item)
                known.add(evidence_id)
        return results

    return agent


class OfflineHistoricalDiagnosisService(DiagnosisService):
    """DiagnosisService variant used only by evals for retired v1 manifests."""

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
        None,
        dict[str, Any],
    ]:
        searcher: EvidenceSearcher | None = None
        if user_id and self._evidence_searcher_factory is not None:
            searcher = self._evidence_searcher_factory(user_id)

        deps = DiagnosisDependencies(
            user_id=user_id,
            body_state_revision=body_state_revision,
            body_state=body_state,
            relevant_history=relevant_history,
            profile=profile,
            evidence_searcher=searcher,
            evidence_acquirer=None,
        )
        result = await _create_retired_v1_agent(config=config).run(
            "Synthesize all supported possible-diagnosis candidates from the pinned durable state.",
            deps=deps,
            model=self._model_resolver(config),
        )
        return (
            result.output,
            _dedupe_evidence(deps.retrieved_evidence),
            None,
            _execution_provenance(result, config),
        )
