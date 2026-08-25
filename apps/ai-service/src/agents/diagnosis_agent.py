"""Typed PydanticAI execution boundary for BodyState Diagnosis."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..agents.evidence import DIAGNOSIS_EVIDENCE_POLICY_V2
from ..models.diagnosis import (
    DIAGNOSIS_OUTPUT_SCHEMA_REVISION,
    DiagnosisAgentOutput,
    DiagnosisDependencies,
    get_diagnosis_output_type,
)
from ..models.evidence import EvidenceGap
from ..prompts.diagnosis import DIAGNOSIS_PROMPT_REVISION, get_diagnosis_system_prompt

DIAGNOSIS_TOOL_POLICY_REVISION = "diagnosis-tools-legacy-v1"
DIAGNOSIS_EVIDENCE_POLICY_REVISION = "diagnosis-evidence-legacy-v1"
DIAGNOSIS_TOOL_POLICY_V2 = "diagnosis-evidence-acquisition-tools-v2"

_SUPPORTED_POLICY_PAIRS = {
    (DIAGNOSIS_TOOL_POLICY_REVISION, DIAGNOSIS_EVIDENCE_POLICY_REVISION),
    (DIAGNOSIS_TOOL_POLICY_V2, DIAGNOSIS_EVIDENCE_POLICY_V2),
}


def diagnosis_tool_names(tool_policy_revision: str) -> list[str]:
    """Return the concrete PydanticAI tool surface for one immutable tool policy."""

    if tool_policy_revision == DIAGNOSIS_TOOL_POLICY_REVISION:
        return ["search_evidence"]
    if tool_policy_revision == DIAGNOSIS_TOOL_POLICY_V2:
        return ["acquire_evidence"]
    raise ValueError(f"unsupported Diagnosis tool policy revision: {tool_policy_revision}")


def create_diagnosis_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = DIAGNOSIS_PROMPT_REVISION,
    output_schema_revision: str = DIAGNOSIS_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = DIAGNOSIS_TOOL_POLICY_REVISION,
    evidence_policy_revision: str = DIAGNOSIS_EVIDENCE_POLICY_REVISION,
) -> Agent[DiagnosisDependencies, DiagnosisAgentOutput]:
    """Create one immutable-config-aware production Diagnosis Agent."""

    supported_tools = {DIAGNOSIS_TOOL_POLICY_REVISION, DIAGNOSIS_TOOL_POLICY_V2}
    supported_evidence = {DIAGNOSIS_EVIDENCE_POLICY_REVISION, DIAGNOSIS_EVIDENCE_POLICY_V2}
    if tool_policy_revision not in supported_tools:
        raise ValueError(f"unsupported Diagnosis tool policy revision: {tool_policy_revision}")
    if evidence_policy_revision not in supported_evidence:
        raise ValueError(
            f"unsupported Diagnosis evidence policy revision: {evidence_policy_revision}"
        )
    if (tool_policy_revision, evidence_policy_revision) not in _SUPPORTED_POLICY_PAIRS:
        raise ValueError(
            "incompatible Diagnosis tool/evidence policy pair: "
            f"{tool_policy_revision} / {evidence_policy_revision}"
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
        evidence_instruction = (
            "Use search_evidence only for a concrete evidence gap that materially "
            "affects a candidate. Do not search merely to decorate the response."
            if tool_policy_revision == DIAGNOSIS_TOOL_POLICY_REVISION
            else (
                "Use acquire_evidence only for a typed, material EvidenceGap. "
                "User facts must use kind=user_fact and can never be supplied by RAG."
            )
        )
        return (
            "Analyze the exact durable BodyState revision supplied for this run.\n"
            f"BodyState revision: R{deps.body_state_revision}\n"
            f"BodyState JSON: {json.dumps(deps.body_state, ensure_ascii=False)}\n"
            f"Relevant history JSON: {json.dumps(deps.relevant_history, ensure_ascii=False)}\n"
            f"Profile JSON: {json.dumps(deps.profile, ensure_ascii=False)}\n"
            f"{evidence_instruction}"
        )

    if tool_policy_revision == DIAGNOSIS_TOOL_POLICY_REVISION:

        @agent.tool
        async def search_evidence(
            ctx: RunContext[DiagnosisDependencies],
            query: str,
            top_k: int = 5,
        ) -> list[dict[str, object]]:
            """Retrieve targeted knowledge for a legacy Diagnosis evidence gap."""

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
            ctx: RunContext[DiagnosisDependencies],
            gap: EvidenceGap,
            top_k: int = 5,
        ) -> dict[str, object]:
            """Resolve one typed EvidenceGap through the bounded acquisition policy."""

            acquirer = ctx.deps.evidence_acquirer
            if acquirer is None:
                raise RuntimeError("controlled Diagnosis evidence acquisition is not configured")
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
