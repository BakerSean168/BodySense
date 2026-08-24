"""Typed PydanticAI Treatment proposal Agent."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.evidence import EvidenceGap
from ..models.treatment import (
    TREATMENT_OUTPUT_SCHEMA_REVISION,
    TreatmentAgentOutput,
    TreatmentDependencies,
    get_treatment_output_type,
)
from ..prompts.treatment import TREATMENT_PROMPT_REVISION, get_treatment_system_prompt
from .evidence import TREATMENT_EVIDENCE_POLICY_V2

TREATMENT_TOOL_POLICY_REVISION = "treatment-tools-v1"
TREATMENT_TOOL_POLICY_V2 = "treatment-tools-v2"
TREATMENT_EVIDENCE_POLICY_REVISION = "treatment-evidence-search-v1"


def treatment_tool_names(tool_policy_revision: str) -> list[str]:
    if tool_policy_revision == TREATMENT_TOOL_POLICY_REVISION:
        return ["search_evidence"]
    if tool_policy_revision == TREATMENT_TOOL_POLICY_V2:
        return ["acquire_evidence"]
    raise ValueError(f"unsupported Treatment tool policy revision: {tool_policy_revision}")


def create_treatment_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = TREATMENT_PROMPT_REVISION,
    output_schema_revision: str = TREATMENT_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = TREATMENT_TOOL_POLICY_REVISION,
    evidence_policy_revision: str = TREATMENT_EVIDENCE_POLICY_REVISION,
) -> Agent[TreatmentDependencies, TreatmentAgentOutput]:
    valid_pairs = {
        (TREATMENT_TOOL_POLICY_REVISION, TREATMENT_EVIDENCE_POLICY_REVISION),
        (TREATMENT_TOOL_POLICY_V2, TREATMENT_EVIDENCE_POLICY_V2),
    }
    if (tool_policy_revision, evidence_policy_revision) not in valid_pairs:
        if tool_policy_revision not in {
            TREATMENT_TOOL_POLICY_REVISION,
            TREATMENT_TOOL_POLICY_V2,
        }:
            raise ValueError(f"unsupported Treatment tool policy revision: {tool_policy_revision}")
        if evidence_policy_revision not in {
            TREATMENT_EVIDENCE_POLICY_REVISION,
            TREATMENT_EVIDENCE_POLICY_V2,
        }:
            raise ValueError(
                f"unsupported Treatment evidence policy revision: {evidence_policy_revision}"
            )
        raise ValueError(
            "incompatible Treatment tool/evidence policy pair: "
            f"{tool_policy_revision} / {evidence_policy_revision}"
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
        evidence_instruction = (
            "Use search_evidence only for a concrete evidence gap that materially "
            "changes an intervention. Do not search merely to decorate the proposal."
            if tool_policy_revision == TREATMENT_TOOL_POLICY_REVISION
            else (
                "Use acquire_evidence only for a typed, material EvidenceGap. "
                "User facts must use kind=user_fact and can never be supplied by RAG."
            )
        )
        return (
            f"Pinned BodyState revision: R{deps.body_state_revision}\n"
            f"BodyState: {json.dumps(deps.body_state, ensure_ascii=False)}\n"
            f"DiagnosisAnalysis: {json.dumps(deps.diagnosis_analysis, ensure_ascii=False)}\n"
            f"Candidate assessments: {json.dumps(deps.candidate_assessments, ensure_ascii=False)}\n"
            f"Profile: {json.dumps(deps.profile, ensure_ascii=False)}\n"
            f"User constraints: {json.dumps(deps.user_constraints, ensure_ascii=False)}\n"
            f"Existing evidence: {json.dumps(deps.evidence, ensure_ascii=False)}\n"
            f"{evidence_instruction}"
        )

    if tool_policy_revision == TREATMENT_TOOL_POLICY_REVISION:

        @agent.tool
        async def search_evidence(
            ctx: RunContext[TreatmentDependencies], query: str, top_k: int = 5
        ) -> list[dict[str, object]]:
            """Retrieve targeted knowledge for the immutable Treatment v1 path."""

            searcher = ctx.deps.evidence_searcher
            if searcher is None:
                return []
            outcome = await searcher.search(query, top_k=top_k)
            results = [dict(item) for item in outcome.evidence]
            _append_evidence(ctx.deps.retrieved_evidence, results)
            return results

    else:

        @agent.tool
        async def acquire_evidence(
            ctx: RunContext[TreatmentDependencies],
            gap: EvidenceGap,
            top_k: int = 5,
        ) -> dict[str, object]:
            """Resolve one typed Treatment EvidenceGap through the bounded policy."""

            acquirer = ctx.deps.evidence_acquirer
            if acquirer is None:
                raise RuntimeError("controlled Treatment evidence acquisition is not configured")
            result = await acquirer.acquire(gap, top_k=top_k)
            _append_evidence(ctx.deps.retrieved_evidence, result.evidence)
            return result.model_dump(mode="json")

    return agent


def _append_evidence(
    destination: list[dict[str, object]], results: list[dict[str, object]]
) -> None:
    known = {str(item.get("evidence_id", "")) for item in destination}
    for item in results:
        evidence_id = str(item.get("evidence_id", ""))
        if evidence_id and evidence_id not in known:
            destination.append(item)
            known.add(evidence_id)
