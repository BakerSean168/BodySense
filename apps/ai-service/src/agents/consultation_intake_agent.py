"""Typed PydanticAI Agent for latest-turn Consultation state acquisition."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.consultation_intake import (
    CONSULTATION_INTAKE_OUTPUT_SCHEMA_REVISION,
    ConsultationIntakeDependencies,
    ConsultationIntakeOutput,
    get_consultation_intake_output_type,
)
from ..prompts.consultation_intake import (
    CONSULTATION_INTAKE_PROMPT_REVISION,
    get_consultation_intake_system_prompt,
)

CONSULTATION_INTAKE_POLICY_REVISION = "consultation-state-acquisition-v1"


def create_consultation_intake_agent(
    model: Model | str | None = None,
    *,
    prompt_revision: str = CONSULTATION_INTAKE_PROMPT_REVISION,
    output_schema_revision: str = CONSULTATION_INTAKE_OUTPUT_SCHEMA_REVISION,
    policy_revision: str = CONSULTATION_INTAKE_POLICY_REVISION,
) -> Agent[ConsultationIntakeDependencies, ConsultationIntakeOutput]:
    if policy_revision != CONSULTATION_INTAKE_POLICY_REVISION:
        raise ValueError(f"unsupported Consultation intake policy revision: {policy_revision}")

    agent: Agent[ConsultationIntakeDependencies, ConsultationIntakeOutput] = Agent(
        model,
        deps_type=ConsultationIntakeDependencies,
        output_type=get_consultation_intake_output_type(output_schema_revision),
        system_prompt=get_consultation_intake_system_prompt(prompt_revision),
        name="bodysense_consultation_intake",
        retries=2,
    )

    @agent.instructions
    def latest_turn_context(ctx: RunContext[ConsultationIntakeDependencies]) -> str:
        deps = ctx.deps
        return (
            "## 本轮最新用户消息（唯一可新建状态的文本）\n"
            f"{deps.latest_user_message}\n\n"
            "## 稳定用户档案（仅作背景）\n"
            f"{json.dumps(deps.profile, ensure_ascii=False)}\n\n"
            "## 当前已确认 BodyState（只用于识别更新/纠正，不得复制）\n"
            f"{json.dumps(deps.body_state, ensure_ascii=False)}"
        )

    return agent
