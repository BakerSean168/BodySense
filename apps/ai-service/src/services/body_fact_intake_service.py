"""Latest-turn extraction for the source-bound BodyFact vertical.

The model proposes original self-report spans; persistence and its receipt are
owned exclusively by Go. Failure never falls back to guessed positive symptoms.
"""

from __future__ import annotations

from collections.abc import Callable

from pydantic_ai import Agent, ModelRetry, RunContext
from pydantic_ai.models import Model

from ..ai.consultation_intake_gateway_model import (
    consultation_intake_model_settings,
    get_consultation_intake_runtime_model,
)
from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.body_fact_intake import BodyFactIntakeOutput, validate_fact_proposal

BODY_FACT_PROMPT = """
你是 BodySense 的原话事实抽取器，不是医生，不负责数据库保存。
只从本轮最新用户原文抽取明确的本人陈述，返回 BodyFactIntakeOutput。
规则：
- turn_kind=self_report
仅用于本人明确事实；general_question/other/ambiguous/correction 不产生 facts。
- statement 必须逐字保留完整原句，包括主体、否定、条件和时间。
  不要转述，不要从引号、第三人称、假设或知识问题中摘出半句。
- 主体必须明确以“我/本人/I/My”开始；“我的朋友/妈妈”等不是本人。
  主体不清楚时返回 ambiguous，并给一个简短澄清问题。
- kind 只选
symptom/health_history/activity/lifestyle/training_feedback。用户怀疑疾病不是既定事实，当前工具不处理假设或诊断。
- “我没有肩痛”是明确的否定事实，presence=absent；
  不知道存在与否用 unknown。非 symptom 不设置 presence。
- 时间只复制原句中的表达，不推算当前日期，不编造部位ID、严重度、恢复、临床诊断或其他字段。
- 明确更正或撤回旧记录属于 correction，不追加相反的新事实假装更正成功。
- 不同句子可以组成一次语义批次；单句不要为了逐个字段而复制多条事实。
- 只输出结构化候选，不承诺“已保存”。最多20项，无需为了未知的严重度或持续时间阻断已经明确的事实。
""".strip()


class BodyFactIntakeService:
    def __init__(
        self, model_resolver: Callable[[ConsultationAgentManifest], Model] | None = None
    ) -> None:
        self._model_resolver = model_resolver or get_consultation_intake_runtime_model

    async def assess(
        self, *, latest_user_message: str, config: ConsultationAgentManifest
    ) -> BodyFactIntakeOutput:
        agent: Agent[str, BodyFactIntakeOutput] = Agent(
            self._model_resolver(config),
            deps_type=str,
            output_type=BodyFactIntakeOutput,
            system_prompt=BODY_FACT_PROMPT,
            name="bodysense_body_fact_intake_v1",
            retries=2,
        )

        @agent.output_validator
        def preserve_source(
            ctx: RunContext[str], output: BodyFactIntakeOutput
        ) -> BodyFactIntakeOutput:
            try:
                validate_fact_proposal(output, ctx.deps)
            except ValueError as error:
                raise ModelRetry(str(error)) from error
            return output

        result = await agent.run(
            latest_user_message,
            deps=latest_user_message,
            model_settings=consultation_intake_model_settings(config),
        )
        return result.output
