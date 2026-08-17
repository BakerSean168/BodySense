"""Human-readable context helpers for observation-only assessment."""

from __future__ import annotations

from typing import Any

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
) -> str:
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
