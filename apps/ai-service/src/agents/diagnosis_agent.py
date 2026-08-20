"""Typed PydanticAI execution boundary for BodyState Diagnosis."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.diagnosis import (
    DIAGNOSIS_OUTPUT_SCHEMA_REVISION,
    DiagnosisAgentOutput,
    DiagnosisDependencies,
    get_diagnosis_output_type,
)
from ..prompts.diagnosis import DIAGNOSIS_PROMPT_REVISION, get_diagnosis_system_prompt

DIAGNOSIS_TOOL_POLICY_REVISION = "diagnosis-tools-legacy-v1"
DIAGNOSIS_EVIDENCE_POLICY_REVISION = "diagnosis-evidence-legacy-v1"


def create_diagnosis_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = DIAGNOSIS_PROMPT_REVISION,
    output_schema_revision: str = DIAGNOSIS_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = DIAGNOSIS_TOOL_POLICY_REVISION,
    evidence_policy_revision: str = DIAGNOSIS_EVIDENCE_POLICY_REVISION,
) -> Agent[DiagnosisDependencies, DiagnosisAgentOutput]:
    """Create the production typed Diagnosis Agent.

    Model selection remains injectable: tests pass ``TestModel`` while production
    supplies the configured fallback model at ``run`` time.
    """

    if tool_policy_revision != DIAGNOSIS_TOOL_POLICY_REVISION:
        raise ValueError(f"unsupported Diagnosis tool policy revision: {tool_policy_revision}")
    if evidence_policy_revision != DIAGNOSIS_EVIDENCE_POLICY_REVISION:
        raise ValueError(
            f"unsupported Diagnosis evidence policy revision: {evidence_policy_revision}"
        )

    agent = Agent(
        model,
        deps_type=DiagnosisDependencies,
        output_type=get_diagnosis_output_type(output_schema_revision),
        system_prompt=get_diagnosis_system_prompt(prompt_revision),
        name="bodysense_diagnosis",
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
            f"Preloaded evidence context: {deps.rag_context or '(none)'}\n"
            "Use search_evidence only for a concrete evidence gap that materially "
            "affects a candidate. Do not search merely to decorate the response."
        )

    @agent.tool
    async def search_evidence(
        ctx: RunContext[DiagnosisDependencies],
        query: str,
        top_k: int = 5,
    ) -> list[dict[str, object]]:
        """Retrieve targeted knowledge for an explicit Diagnosis evidence gap."""

        searcher = ctx.deps.evidence_searcher
        if searcher is None:
            return []
        results = await searcher.search(query, top_k=top_k)
        known = {str(item.get("evidence_id", "")) for item in ctx.deps.retrieved_evidence}
        for item in results:
            evidence_id = str(item.get("evidence_id", ""))
            if evidence_id and evidence_id not in known:
                ctx.deps.retrieved_evidence.append(item)
                known.add(evidence_id)
        return results

    return agent
