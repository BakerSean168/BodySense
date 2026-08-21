"""PydanticAI Agent for observation-only health assessment."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.assessment import (
    ASSESSMENT_OUTPUT_SCHEMA_REVISION,
    AssessmentAgentOutput,
    AssessmentDependencies,
    get_assessment_output_type,
)
from ..prompts.assessment import (
    ASSESSMENT_PROMPT_REVISION,
    ASSESSMENT_PROMPT_REVISION_V2,
    get_assessment_system_prompt,
)

ASSESSMENT_TOOL_POLICY_REVISION = "assessment-tools-none-v1"

_SUPPORTED_ASSESSMENT_PROMPT_REVISIONS = {
    ASSESSMENT_PROMPT_REVISION,
    ASSESSMENT_PROMPT_REVISION_V2,
}


def create_assessment_agent(
    model: Model | None = None,
    *,
    prompt_revision: str = ASSESSMENT_PROMPT_REVISION,
    output_schema_revision: str = ASSESSMENT_OUTPUT_SCHEMA_REVISION,
    tool_policy_revision: str = ASSESSMENT_TOOL_POLICY_REVISION,
) -> Agent[AssessmentDependencies, AssessmentAgentOutput]:
    if prompt_revision not in _SUPPORTED_ASSESSMENT_PROMPT_REVISIONS:
        raise ValueError(f"unsupported Assessment prompt revision: {prompt_revision}")
    if tool_policy_revision != ASSESSMENT_TOOL_POLICY_REVISION:
        raise ValueError(
            f"unsupported Assessment tool policy revision: {tool_policy_revision}"
        )

    agent: Agent[AssessmentDependencies, AssessmentAgentOutput] = Agent(
        model,
        deps_type=AssessmentDependencies,
        output_type=get_assessment_output_type(output_schema_revision),
        system_prompt=get_assessment_system_prompt(prompt_revision),
        name="bodysense_assessment",
    )

    @agent.instructions
    def assessment_context(ctx: RunContext[AssessmentDependencies]) -> str:
        deps = ctx.deps
        sections = [
            "## 用户档案（仅作为观察背景）\n" + json.dumps(deps.profile, ensure_ascii=False),
        ]
        if deps.posture_analysis:
            sections.append(
                "## 已完成的体态图片分析（复用既有 observation evidence）\n"
                + json.dumps(deps.posture_analysis, ensure_ascii=False)
            )
        if deps.rag_context:
            sections.append("## 参考上下文\n" + deps.rag_context)
        return "\n\n".join(sections)

    return agent
