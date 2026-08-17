"""PydanticAI Agent for observation-only health assessment."""

from __future__ import annotations

import json

from pydantic_ai import Agent, RunContext
from pydantic_ai.models import Model

from ..models.assessment import AssessmentAgentOutput, AssessmentDependencies

ASSESSMENT_SYSTEM_PROMPT = """你是 BodySense 的结构化观察评估 Agent。

你的职责仅限于：
1. 根据用户档案、已有体态分析和本次图片，形成可由用户审核的观察候选；
2. 给出描述性的维度评分和总结；
3. 明确不确定性、信息缺口和安全提示。

严格边界：
- 不得输出医学诊断；
- 不得输出运动处方、训练计划、营养方案或治疗建议；
- 不得把图片外观推断成已经确认的用户事实；
- observation 是待审核候选，后续由 Go 写入 BodyState，并在用户确认前排除于 Diagnosis reasoning；
- 只描述输入能支持的可见或资料性观察；无法支持时使用 insufficient_information；
- 安全风险只写入 safety_notes，不得弱化就医提醒。
"""


def create_assessment_agent(
    model: Model | None = None,
) -> Agent[AssessmentDependencies, AssessmentAgentOutput]:
    agent: Agent[AssessmentDependencies, AssessmentAgentOutput] = Agent(
        model,
        deps_type=AssessmentDependencies,
        output_type=AssessmentAgentOutput,
        system_prompt=ASSESSMENT_SYSTEM_PROMPT,
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
