"""Typed PydanticAI execution boundary for the current BodyState Diagnosis runtime."""

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
from ..prompts.diagnosis import (
    DIAGNOSIS_CONTEXT_PROMPT_REVISION,
    DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION,
    DIAGNOSIS_PROMPT_REVISION,
    DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION,
    get_diagnosis_system_prompt,
)
from .diagnosis_prompt_context import (
    body_state_prompt_view,
    compact_json,
    history_prompt_view,
    legacy_body_state_prompt_view,
    legacy_history_prompt_view,
    profile_prompt_view,
)

DIAGNOSIS_TOOL_POLICY_V2 = "diagnosis-evidence-acquisition-tools-v2"


def diagnosis_tool_names(tool_policy_revision: str) -> list[str]:
    """Return the concrete tool surface for the current Diagnosis runtime."""

    if tool_policy_revision != DIAGNOSIS_TOOL_POLICY_V2:
        raise ValueError(f"unsupported Diagnosis tool policy revision: {tool_policy_revision}")
    return ["acquire_evidence"]


def create_diagnosis_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = DIAGNOSIS_PROMPT_REVISION,
    output_schema_revision: str = DIAGNOSIS_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = DIAGNOSIS_TOOL_POLICY_V2,
    evidence_policy_revision: str = DIAGNOSIS_EVIDENCE_POLICY_V2,
) -> Agent[DiagnosisDependencies, DiagnosisAgentOutput]:
    """Create the current production Diagnosis Agent.

    Historical tool/evidence policies are intentionally unsupported here. Offline
    qualification of retired manifests is implemented under ``src.evals`` so a
    historical configuration cannot become runtime-addressable by calling this factory.
    """

    if tool_policy_revision != DIAGNOSIS_TOOL_POLICY_V2:
        raise ValueError(f"unsupported Diagnosis tool policy revision: {tool_policy_revision}")
    if evidence_policy_revision != DIAGNOSIS_EVIDENCE_POLICY_V2:
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
        return diagnosis_context_instructions(ctx.deps, prompt_revision)

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


def _legacy_json(value: object) -> str:
    return json.dumps(value, ensure_ascii=False)


def diagnosis_context_instructions(deps: DiagnosisDependencies, prompt_revision: str) -> str:
    """Serialize an explicitly versioned model-facing Diagnosis context."""

    legacy_revisions = {DIAGNOSIS_PROMPT_REVISION, DIAGNOSIS_EVIDENCE_GAP_PROMPT_REVISION}
    if prompt_revision in legacy_revisions:
        body = legacy_body_state_prompt_view(deps.body_state)
        history = legacy_history_prompt_view(deps.relevant_history)
        profile = deps.profile
        serialize = _legacy_json
        requires_envelope = False
    elif prompt_revision == DIAGNOSIS_STRUCTURED_SAFETY_PROMPT_REVISION:
        # v8 is frozen byte-for-byte for historical replay evidence.
        body = deps.body_state
        history = deps.relevant_history
        profile = deps.profile
        serialize = _legacy_json
        requires_envelope = True
    elif prompt_revision == DIAGNOSIS_CONTEXT_PROMPT_REVISION:
        body = body_state_prompt_view(deps.body_state)
        history = history_prompt_view(deps.relevant_history)
        profile = profile_prompt_view(deps.profile)
        serialize = compact_json
        requires_envelope = True
    else:
        raise ValueError(f"unsupported Diagnosis prompt revision: {prompt_revision}")

    instructions = (
        "Analyze the exact durable BodyState revision supplied for this run.\n"
        f"BodyState revision: R{deps.body_state_revision}\n"
        f"BodyState JSON: {serialize(body)}\n"
        f"Relevant history JSON: {serialize(history)}\n"
        f"Profile JSON: {serialize(profile)}\n"
        "Use acquire_evidence only for a typed, material EvidenceGap. "
        "User facts must use kind=user_fact and can never be supplied by RAG."
    )

    if requires_envelope:
        if deps.safety_envelope is None:
            raise ValueError("Structured Diagnosis requires SafetyEnvelopeV2")
        instructions += "\nSafetyEnvelopeV2 JSON: " + deps.safety_envelope.model_dump_json()
    return instructions
