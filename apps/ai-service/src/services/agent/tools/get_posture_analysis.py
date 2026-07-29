"""get_posture_analysis tool — reads the user's completed posture analysis.

The Go runtime prefetches ``posture_analysis`` into the consultation business
context (and therefore into thread state). This tool never re-runs the VLM;
it only surfaces already-persisted ``user_uploads.analysis_result`` rows so
the Agent can ground advice in the user's photo findings.
"""

from __future__ import annotations

from typing import Any

from ..tool_types import RuntimeToolDefinition, ToolCategory

GET_POSTURE_ANALYSIS_SCHEMA: dict[str, Any] = {
    "name": "get_posture_analysis",
    "description": (
        "读取用户已完成的三视角体态照片分析结果。"
        "当用户询问自己的体态问题、需要结合照片发现做判断、"
        "或你想引用已有的体态评估发现时调用此工具。"
        "不要要求用户重新上传照片；若无已完成分析，如实告知即可。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "view": {
                "type": "string",
                "description": "可选，仅返回某一视角：front / side / back。省略则返回全部。",
                "enum": ["front", "side", "back"],
            },
        },
        "required": [],
    },
}


def format_posture_analysis_for_llm(summary: dict[str, Any], view: str | None = None) -> str:
    """Turn a posture analysis summary into readable tool text for the LLM."""
    if not summary or not summary.get("has_analysis"):
        return (
            "【体态分析】当前用户没有已完成的体态照片分析。"
            "可建议用户在档案页上传正面/侧面/背面站姿照片后再引用。"
        )

    views = list(summary.get("views") or [])
    if view:
        views = [v for v in views if str(v.get("view", "")) == view]
        if not views:
            return f"【体态分析】未找到视角 `{view}` 的已完成分析。"

    parts: list[str] = ["【体态分析】已读取用户已完成的体态照片分析："]
    for item in views:
        view_name = item.get("view") or item.get("file_type") or "unknown"
        analysis = item.get("analysis") or {}
        if isinstance(analysis, str):
            # Defensive: some transports may leave raw JSON strings.
            parts.append(f"- {view_name}：{analysis[:400]}")
            continue

        confidence = analysis.get("overall_confidence", "")
        header = f"- 视角 {view_name}"
        if confidence:
            header += f"（置信度：{confidence}）"
        parts.append(header)

        findings = analysis.get("findings") or []
        for finding in findings[:8]:
            if not isinstance(finding, dict):
                continue
            label = finding.get("label") or finding.get("key") or "发现"
            severity = finding.get("severity", "")
            evidence = finding.get("evidence", "")
            line = f"  · {label}"
            if severity:
                line += f"（{severity}）"
            if evidence:
                line += f"：{evidence}"
            parts.append(line)

        summary_md = (analysis.get("summary_markdown") or "").strip()
        if summary_md:
            parts.append(f"  摘要：{summary_md[:280]}")

        red_flags = analysis.get("red_flags") or []
        for flag in red_flags[:3]:
            if isinstance(flag, dict):
                parts.append(f"  ⚠ {flag.get('message') or flag.get('category')}")

    flat_findings = summary.get("findings") or []
    if flat_findings and not any("·" in p for p in parts):
        parts.append(f"- 共 {len(flat_findings)} 条发现（详见 structured result）")

    parts.append("以上来自已存储的 analysis_result，不是本轮重新视觉分析。")
    return "\n".join(parts)


def read_posture_analysis(
    posture_analysis: dict[str, Any] | None,
    *,
    view: str | None = None,
) -> dict[str, Any]:
    """Pure reader used by the tool handler and unit tests.

    Returns a structured dict the runtime can put on the tool message and
    optionally surface as a citation-like card.
    """
    summary = posture_analysis if isinstance(posture_analysis, dict) else {}
    empty = {"has_analysis": False, "views": [], "findings": [], "summaries": []}
    filtered = dict(summary) if summary else empty

    if view and filtered.get("views"):
        filtered = {
            **filtered,
            "views": [v for v in filtered["views"] if str(v.get("view", "")) == view],
        }
        filtered["has_analysis"] = bool(filtered["views"])

    result_text = format_posture_analysis_for_llm(summary, view=view)
    return {
        "result_text": result_text,
        "has_analysis": bool(filtered.get("has_analysis")),
        "summary": filtered,
        "view_filter": view,
    }


async def handle_get_posture_analysis(arguments: dict[str, Any]) -> dict[str, Any]:
    """Tool handler.

    The live runtime injects ``_posture_analysis`` into arguments before
    dispatch (see ``execute_tool``). Direct unit calls may pass it explicitly.
    """
    view = arguments.get("view")
    if view is not None:
        view = str(view).strip() or None
    posture_analysis = arguments.get("_posture_analysis")
    if posture_analysis is None:
        # Also accept a top-level summary if tests pass it that way.
        posture_analysis = arguments.get("posture_analysis")
    return read_posture_analysis(
        posture_analysis if isinstance(posture_analysis, dict) else None,
        view=view,
    )


def make_get_posture_analysis_tool() -> RuntimeToolDefinition:
    """Create a RuntimeToolDefinition for get_posture_analysis."""
    return RuntimeToolDefinition(
        name=GET_POSTURE_ANALYSIS_SCHEMA["name"],
        description=GET_POSTURE_ANALYSIS_SCHEMA["description"],
        parameters=GET_POSTURE_ANALYSIS_SCHEMA["parameters"],
        category=ToolCategory.QUERY,
        handler=handle_get_posture_analysis,
        required_params=[],
    )
