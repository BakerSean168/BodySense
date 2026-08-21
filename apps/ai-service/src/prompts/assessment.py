"""Human-readable context helpers for observation-only assessment."""

from __future__ import annotations

from typing import Any

ASSESSMENT_PROMPT_REVISION = "assessment-prompt-v1"
ASSESSMENT_PROMPT_REVISION_V2 = "assessment-prompt-v2"

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

# v2 iterates the observation taxonomy: it asks the Agent to split observations
# into fine-grained posture/lifestyle/exercise-influence categories and to tag
# each observation with the evidence source (photo, profile, report) it relies
# on. It keeps the same output schema, tool surface, and boundaries as v1 so it
# can be qualified and rollout-gated as a Challenger without changing Go.
ASSESSMENT_SYSTEM_PROMPT_V2 = """你是 BodySense 的结构化观察评估 Agent（v2）。

你的职责仅限于：
1. 根据用户档案、已有体态分析和本次图片，形成可由用户审核的观察候选；
2. 给出描述性的维度评分和总结；
3. 明确不确定性、信息缺口和安全提示。

v2 细化：
- 每条 observation 使用更细的 kind（如 posture_alignment / posture_asymmetry /
  lifestyle_pattern / exercise_influence / body_fat_distribution），并在
  condition.evidence 标注依据来源（photo/profile/report）；
- 明确区分「图片可见事实」与「资料推断」，后者必须标 confidence=低 并写入 information_gaps；
- 信息缺口要具体到缺失的资料类型（如“缺少侧面照片”而不是“信息不足”）。

严格边界（与 v1 一致）：
- 不得输出医学诊断；
- 不得输出运动处方、训练计划、营养方案或治疗建议；
- observation 是待审核候选，后续由 Go 写入 BodyState，并在用户确认前排除于 Diagnosis reasoning；
- 安全风险只写入 safety_notes，不得弱化就医提醒。
"""

ASSESSMENT_DISCLAIMER = (
    "本报告只呈现待审核的资料与体态观察，不构成医疗诊断、治疗方案或运动处方。"
    "如存在持续疼痛、进行性无力、麻木或严重不适，请寻求专业医疗评估。"
)



def format_posture_analysis_section(posture_analysis: dict[str, Any] | None) -> str:
    if not isinstance(posture_analysis, dict) or not posture_analysis.get("has_analysis"):
        return ""

    lines = ["## 已完成的体态照片分析（复用 analysis_result，不重新臆测角度）"]
    for view in posture_analysis.get("views") or []:
        if not isinstance(view, dict):
            continue
        view_name = view.get("view") or view.get("file_type") or "unknown"
        analysis = view.get("analysis") or {}
        if not isinstance(analysis, dict):
            lines.append(f"- 视角 {view_name}：已有分析")
            continue
        confidence = analysis.get("overall_confidence", "")
        header = f"- 视角 {view_name}"
        if confidence:
            header += f"（置信度：{confidence}）"
        lines.append(header)
        for finding in (analysis.get("findings") or [])[:8]:
            if not isinstance(finding, dict):
                continue
            label = finding.get("label") or finding.get("key") or "发现"
            severity = finding.get("severity", "")
            evidence = finding.get("evidence", "")
            item = f"  · {label}"
            if severity:
                item += f"（{severity}）"
            if evidence:
                item += f"：{evidence}"
            lines.append(item)
        summary_md = str(analysis.get("summary_markdown") or "").strip()
        if summary_md:
            lines.append(f"  摘要：{summary_md[:280]}")

    for text in posture_analysis.get("summaries") or []:
        if text and not any(str(text)[:40] in line for line in lines):
            lines.append(f"- 综合摘要：{str(text)[:280]}")

    lines.append("这些内容只能生成待用户审核的 Observation，不得直接形成 Diagnosis 或 Treatment。")
    return "\n".join(lines)


def get_assessment_prompt(
    profile: dict[str, Any],
    rag_context: str = "",
    posture_analysis: dict[str, Any] | None = None,
    *,
    prompt_revision: str = ASSESSMENT_PROMPT_REVISION,
) -> str:
    if prompt_revision != ASSESSMENT_PROMPT_REVISION:
        raise ValueError(f"unsupported Assessment prompt revision: {prompt_revision}")
    parts = ["请生成一份 observation-only 评估报告。"]
    if profile:
        parts.append("用户档案已通过 run dependencies 提供。")
    posture_section = format_posture_analysis_section(posture_analysis)
    if posture_section:
        parts.append(posture_section)
    if rag_context:
        parts.append("参考上下文已通过 run dependencies 提供。")
    parts.append(ASSESSMENT_DISCLAIMER)
    return "\n\n".join(parts)


def get_assessment_system_prompt(revision: str = ASSESSMENT_PROMPT_REVISION) -> str:
    """Return the deterministic system prompt for a supported prompt revision."""
    prompts = {
        ASSESSMENT_PROMPT_REVISION: ASSESSMENT_SYSTEM_PROMPT,
        ASSESSMENT_PROMPT_REVISION_V2: ASSESSMENT_SYSTEM_PROMPT_V2,
    }
    try:
        return prompts[revision]
    except KeyError as exc:
        raise ValueError(f"unsupported Assessment prompt revision: {revision}") from exc
