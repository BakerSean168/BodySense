"""Typed PydanticAI Treatment proposal Agent."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.treatment import (
    TREATMENT_OUTPUT_SCHEMA_REVISION,
    TreatmentAgentOutput,
    TreatmentDependencies,
    get_treatment_output_type,
)
from ..prompts.treatment import TREATMENT_PROMPT_REVISION, get_treatment_system_prompt

TREATMENT_TOOL_POLICY_REVISION = "treatment-tools-v1"
TREATMENT_EVIDENCE_POLICY_REVISION = "treatment-evidence-search-v1"


def treatment_tool_names(tool_policy_revision: str) -> list[str]:
    if tool_policy_revision != TREATMENT_TOOL_POLICY_REVISION:
        raise ValueError(f"unsupported Treatment tool policy revision: {tool_policy_revision}")
    return ["search_evidence"]


def create_treatment_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = TREATMENT_PROMPT_REVISION,
    output_schema_revision: str = TREATMENT_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = TREATMENT_TOOL_POLICY_REVISION,
    evidence_policy_revision: str = TREATMENT_EVIDENCE_POLICY_REVISION,
) -> Agent[TreatmentDependencies, TreatmentAgentOutput]:
    if tool_policy_revision != TREATMENT_TOOL_POLICY_REVISION:
        raise ValueError(f"unsupported Treatment tool policy revision: {tool_policy_revision}")
    if evidence_policy_revision != TREATMENT_EVIDENCE_POLICY_REVISION:
        raise ValueError(
            f"unsupported Treatment evidence policy revision: {evidence_policy_revision}"
        )

    agent = Agent(
        model,
        deps_type=TreatmentDependencies,
        output_type=get_treatment_output_type(output_schema_revision),
        system_prompt=get_treatment_system_prompt(prompt_revision),
        name="bodysense_treatment",
        retries=2,
    )

    @agent.instructions
    def durable_context(ctx: RunContext[TreatmentDependencies]) -> str:
        deps = ctx.deps
        return (
            f"Pinned BodyState revision: R{deps.body_state_revision}\n"
            f"BodyState: {json.dumps(deps.body_state, ensure_ascii=False)}\n"
            f"DiagnosisAnalysis: {json.dumps(deps.diagnosis_analysis, ensure_ascii=False)}\n"
            f"Candidate assessments: {json.dumps(deps.candidate_assessments, ensure_ascii=False)}\n"
            f"Profile: {json.dumps(deps.profile, ensure_ascii=False)}\n"
            f"User constraints: {json.dumps(deps.user_constraints, ensure_ascii=False)}\n"
            f"Existing evidence: {json.dumps(deps.evidence, ensure_ascii=False)}"
        )

    @agent.tool
    async def search_evidence(
        ctx: RunContext[TreatmentDependencies], query: str, top_k: int = 5
    ) -> list[dict[str, object]]:
        """Retrieve evidence only when it materially affects an intervention."""

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
