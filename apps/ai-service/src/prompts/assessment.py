"""Human-readable context helpers for observation-only assessment."""

from __future__ import annotations

from typing import Any

ASSESSMENT_PROMPT_REVISION = "assessment-prompt-v1"
ASSESSMENT_PROMPT_REVISION_V2 = "assessment-prompt-v2"
ASSESSMENT_PROMPT_REVISION_V3 = "assessment-prompt-v3-evidence-contract"

ASSESSMENT_SYSTEM_PROMPT = """你是 BodySense 的结构化观察评估 Agent。

你的职责仅限于：
1. 根据稳定用户档案、当前 BodyState、已有体态分析和本次图片，形成可由用户审核的观察候选；
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

ASSESSMENT_SYSTEM_PROMPT_V2 = """你是 BodySense 的结构化观察评估 Agent（v2）。

你的职责仅限于：
1. 根据稳定用户档案、当前 BodyState、已有体态分析和本次图片，形成可由用户审核的观察候选；
2. 给出描述性的维度评分和总结；
3. 明确不确定性、信息缺口和安全提示。

v2 细化：
- 每条 observation 使用更细的 kind（如 posture_alignment / posture_asymmetry /
  lifestyle_pattern / exercise_influence / body_fat_distribution），并在
  condition.evidence 标注依据来源（photo/profile/body_state/report）；
- 明确区分「图片可见事实」与「资料推断」，后者必须标 confidence=低 并写入 information_gaps；
- 信息缺口要具体到缺失的资料类型（如“缺少侧面照片”而不是“信息不足”）。

严格边界（与 v1 一致）：
- 不得输出医学诊断；
- 不得输出运动处方、训练计划、营养方案或治疗建议；
- observation 是待审核候选，后续由 Go 写入 BodyState，并在用户确认前排除于 Diagnosis reasoning；
- 安全风险只写入 safety_notes，不得弱化就医提醒。
"""

ASSESSMENT_SYSTEM_PROMPT_V3 = """你是 BodySense 的 evidence-selection Assessment Agent。

你唯一可以做的事情是：从“可用证据目录”选择证据，并为每条证据指定一个 observation kind。
你不拥有任何可持久化自然语言的撰写权限；label、description、body_region、summary、
健康等级、分数、建议等都由应用层确定性生成或明确禁止。

每条 observation 必须遵守：
- 只输出 kind 和 evidence_refs；不得输出任何其它字段；
- evidence_refs 必须恰好包含 1 个目录中真实存在的 ref；
- 同一个 evidence ref 最多选择一次；
- kind 只能是 posture_alignment、posture_asymmetry、lifestyle_pattern、
  exercise_pattern、report_indicator、anthropometry；
- posture_alignment / posture_asymmetry 只能选择 posture_analysis；
- exercise_pattern 只能选择 BodyState lifestyle.exercise；
- lifestyle_pattern 只能选择 BodyState 中除 exercise 外的 lifestyle.*；
- report_indicator 只能选择 report；
- anthropometry 只能选择 BodyState anthropometry.*；
- 不得根据年龄、性别、久坐、运动频率等间接推导新的身体状态；
- 证据不足时返回 observations=[]。

输出必须严格匹配结构化 schema。
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
    if prompt_revision == ASSESSMENT_PROMPT_REVISION_V3:
        return (
            "请只选择可用证据并输出 kind + 单个 evidence_ref。"
            "不要生成任何观察文案；如果没有足够证据，返回 observations=[]。\n\n"
            + ASSESSMENT_DISCLAIMER
        )
    if prompt_revision not in {ASSESSMENT_PROMPT_REVISION, ASSESSMENT_PROMPT_REVISION_V2}:
        raise ValueError(f"unsupported Assessment prompt revision: {prompt_revision}")

    parts = ["请生成一份 observation-only 评估报告。"]
    if profile:
        parts.append("稳定用户档案已通过 run dependencies 提供。")
    parts.append(
        "当前 BodyState 与报告指标通过 run dependencies 提供；可变健康信息以 BodyState 为准。"
    )
    posture_section = format_posture_analysis_section(posture_analysis)
    if posture_section:
        parts.append(posture_section)
    if rag_context:
        parts.append("参考上下文已通过 run dependencies 提供。")
    parts.append(ASSESSMENT_DISCLAIMER)
    return "\n\n".join(parts)


def get_assessment_system_prompt(revision: str = ASSESSMENT_PROMPT_REVISION) -> str:
    prompts = {
        ASSESSMENT_PROMPT_REVISION: ASSESSMENT_SYSTEM_PROMPT,
        ASSESSMENT_PROMPT_REVISION_V2: ASSESSMENT_SYSTEM_PROMPT_V2,
        ASSESSMENT_PROMPT_REVISION_V3: ASSESSMENT_SYSTEM_PROMPT_V3,
    }
    try:
        return prompts[revision]
    except KeyError as exc:
        raise ValueError(f"unsupported Assessment prompt revision: {revision}") from exc
