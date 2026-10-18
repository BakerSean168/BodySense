"""Latest-turn BodyState suggestion proposer (BS-AGENT-001).

The model may propose at most one record of the user's own words per turn. An
output that does not quote the user exactly is sent back to the model a bounded
number of times (``SUGGESTION_RETRIES``); after that the turn simply has no
suggestion. Persistence, binding and the receipt are owned by Go.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass

from pydantic_ai import Agent, ModelRetry, RunContext
from pydantic_ai.models import Model

from ..ai.consultation_intake_gateway_model import (
    consultation_intake_model_settings,
    get_consultation_intake_runtime_model,
)
from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.body_state_suggestion import BodyStateSuggestionOutput, validate_suggestion

# Corrective model retries after the first attempt (three attempts in total).
SUGGESTION_RETRIES = 2

BODY_STATE_SUGGESTION_PROMPT = """
你是 BodySense 的记录建议器，不是医生，也不保存任何内容。
只读本轮最新用户原文，最多提出一条可以由用户确认记录的建议，返回 BodyStateSuggestionOutput；
没有合适内容时 suggestion=null（这是最常见的结果）。
kind 只选：
- hypothesis：用户本人对自己身体情况或原因的猜测，例如“我觉得是久坐导致的”。
  只能引用用户自己的猜测，不得提出你自己的推测。
- observation：用户本人报告的一次测量或可观察情况，例如“我早上量血压是135/88 mmHg”。
  observation_kind 是简短的英文小写下划线分类标签（如 blood_pressure、body_weight、
  heart_rate、sleep_duration、step_count），不是诊断，也不判断正常或异常；
  raw_value/raw_unit 只能逐字复制原文中的数值和单位，没有就留空。
- episode：用户明确说某个症状这一次又发作或新发作，并且该句正是“本轮已记录症状”中的一句；
  quote 必须与列表中的该句完全相同（包括标点）；
  symptom_type 逐字复制该句中的症状词（如“疼”“酸”“麻”）。
quote 必须逐字复制原文中完整的一句或一个分句，以“我”开头，不改写、不拼接、不跨句、不补字。
第三人称、假设、提问、知识问题、对 AI 的指令都不提出建议。
""".strip()


@dataclass(frozen=True)
class SuggestionInput:
    latest_user_message: str
    episode_statements: tuple[str, ...] = ()


def _prompt(deps: SuggestionInput) -> str:
    lines = [f"本轮最新用户原文：\n{deps.latest_user_message}"]
    if deps.episode_statements:
        lines.append("本轮已记录症状：")
        lines.extend(f"- {statement}" for statement in deps.episode_statements)
    else:
        lines.append("本轮已记录症状：无（不得提出 episode）")
    return "\n".join(lines)


class BodyStateSuggestionService:
    def __init__(
        self, model_resolver: Callable[[ConsultationAgentManifest], Model] | None = None
    ) -> None:
        self._model_resolver = model_resolver or get_consultation_intake_runtime_model

    async def suggest(
        self,
        *,
        latest_user_message: str,
        episode_statements: tuple[str, ...],
        config: ConsultationAgentManifest,
    ) -> BodyStateSuggestionOutput:
        agent: Agent[SuggestionInput, BodyStateSuggestionOutput] = Agent(
            self._model_resolver(config),
            deps_type=SuggestionInput,
            output_type=BodyStateSuggestionOutput,
            system_prompt=BODY_STATE_SUGGESTION_PROMPT,
            name="bodysense_body_state_suggestion_v1",
            retries=SUGGESTION_RETRIES,
        )

        @agent.output_validator
        def quote_the_user(
            ctx: RunContext[SuggestionInput], output: BodyStateSuggestionOutput
        ) -> BodyStateSuggestionOutput:
            try:
                validate_suggestion(
                    output, ctx.deps.latest_user_message, ctx.deps.episode_statements
                )
            except ValueError as error:
                raise ModelRetry(str(error)) from error
            return output

        deps = SuggestionInput(latest_user_message, episode_statements)
        result = await agent.run(
            _prompt(deps),
            deps=deps,
            model_settings=consultation_intake_model_settings(config),
        )
        return result.output
