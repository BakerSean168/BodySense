"""Execute one model-requested consultation tool call."""

from __future__ import annotations

import json
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any

from langgraph.types import StreamWriter, interrupt

from ..services.agent.answer_attribution import (
    build_published_evidence_binding,
    validate_and_evaluate_attribution,
)
from ..services.agent.consultation_tools import ToolExecutor, get_consultation_executor
from ..services.agent.reply_fallback import emit_citation_events
from ..services.agent.tool_types import ToolStatus
from .consultation_thread_manifest import get_consultation_manifest
from .consultation_thread_state import ConsultationThreadState, _merge_symptoms


@dataclass
class _ToolCallContext:
    """One popped tool call plus the runtime accumulators its handler updates."""

    state: ConsultationThreadState
    writer: StreamWriter
    executor: ToolExecutor
    tool_call_id: str
    tool_name: str
    arguments: dict[str, Any]
    runtime_messages: list[Any]
    remaining: list[Any]

    def emit_tool_result(self, result: dict[str, Any]) -> None:
        self.writer(
            {
                "type": "tool_result",
                "id": self.tool_call_id,
                "tool": self.tool_name,
                "result": result,
            }
        )

    def append_tool_message(self, content: str) -> None:
        self.runtime_messages.append(
            {"role": "tool", "tool_call_id": self.tool_call_id, "content": content}
        )

    def base_update(self) -> dict[str, Any]:
        return {
            "runtime_messages": self.runtime_messages,
            "pending_tool_calls": self.remaining,
        }


async def _handle_extract_symptom_info(ctx: _ToolCallContext) -> dict[str, Any]:
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, ctx.arguments)
    normalized = result.content if result.status == ToolStatus.SUCCESS else ctx.arguments
    assert isinstance(normalized, dict), "extract_symptom_info must return a dict"
    ctx.writer({"type": "extracted_info", "info": normalized})
    ctx.emit_tool_result({"status": "ok"})
    ctx.append_tool_message("症状信息已提取。")
    return {
        **ctx.base_update(),
        "extracted_symptoms": _merge_symptoms(
            ctx.state.get("extracted_symptoms", []),
            [normalized],
        ),
    }


async def _handle_record_lifestyle_context(ctx: _ToolCallContext) -> dict[str, Any]:
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, ctx.arguments)
    normalized = result.content if result.status == ToolStatus.SUCCESS else ctx.arguments
    assert isinstance(normalized, dict), "record_lifestyle_context must return a dict"
    ctx.writer({"type": "lifestyle_context", "context": normalized})
    ctx.emit_tool_result({"status": "ok"})
    ctx.append_tool_message("生活方式信息已记录。")
    return ctx.base_update()


async def _handle_search_knowledge(ctx: _ToolCallContext) -> dict[str, Any]:
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, ctx.arguments)
    has_results = False
    retrieved = dict(ctx.state.get("retrieved_published_evidence", {}) or {})
    result_text = result.error or "搜索失败"
    if result.status == ToolStatus.SUCCESS:
        content = result.content if isinstance(result.content, dict) else {}
        result_text = str(content.get("result_text", ""))
        has_results = bool(content.get("has_results", False))
        raw_results = content.get("raw_results", [])
        if has_results:
            emit_citation_events(raw_results, ctx.writer)
            for raw_result in raw_results:
                binding = build_published_evidence_binding(raw_result)
                if binding is not None:
                    retrieved[str(binding["evidence_ref"])] = binding
        else:
            ctx.writer(
                {
                    "type": "knowledge_gap",
                    "query": ctx.arguments.get("query", ""),
                    "message": "知识库中暂未找到相关专项资料，以下为通用建议。",
                }
            )
    ctx.emit_tool_result({"has_results": has_results})
    ctx.append_tool_message(result_text)
    return {**ctx.base_update(), "retrieved_published_evidence": retrieved}


async def _handle_record_answer_attribution(ctx: _ToolCallContext) -> dict[str, Any]:
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, ctx.arguments)
    if result.status != ToolStatus.SUCCESS:
        ctx.emit_tool_result({"status": "error", "error": result.error or "invalid attribution"})
        ctx.append_tool_message(result.error or "归因参数无效，请修正后重试。")
        return ctx.base_update()

    try:
        evaluated = validate_and_evaluate_attribution(
            list(ctx.arguments.get("claims") or []),
            dict(ctx.state.get("retrieved_published_evidence", {}) or {}),
        )
    except ValueError as exc:
        error_text = str(exc)
        ctx.emit_tool_result({"status": "error", "error": error_text})
        ctx.append_tool_message(
            f"归因校验失败：{error_text}。只能使用本轮搜索返回的 Published Evidence Ref。"
        )
        return ctx.base_update()

    existing_attributions = list(ctx.state.get("answer_attributions", []) or [])
    emitted: list[dict[str, Any]] = []
    for index, attribution in enumerate(evaluated):
        payload = {
            **attribution,
            "attribution_id": f"{ctx.tool_call_id}:{index}",
        }
        ctx.writer({"type": "answer_attribution", "attribution": payload})
        emitted.append(payload)
    ctx.emit_tool_result({"status": "ok", "recorded_claims": len(emitted)})
    ctx.append_tool_message("回答证据归因已记录。现在直接给出自然语言回答，不要展示 Evidence Ref。")
    return {**ctx.base_update(), "answer_attributions": existing_attributions + emitted}


async def _handle_get_posture_analysis(ctx: _ToolCallContext) -> dict[str, Any]:
    # Inject prefetched analysis so the pure tool never needs a reverse HTTP call.
    tool_args = dict(ctx.arguments)
    tool_args["_posture_analysis"] = ctx.state.get("posture_analysis") or {
        "has_analysis": False,
        "views": [],
        "findings": [],
        "summaries": [],
    }
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, tool_args)
    has_analysis = False
    result_text = result.error or "读取体态分析失败"
    summary: dict[str, Any] = {}
    if result.status == ToolStatus.SUCCESS:
        content = result.content if isinstance(result.content, dict) else {}
        if isinstance(content, dict):
            result_text = str(content.get("result_text", ""))
            has_analysis = bool(content.get("has_analysis", False))
            summary = content.get("summary") or {}
    ctx.emit_tool_result({"has_analysis": has_analysis, "summary": summary})
    ctx.append_tool_message(result_text)
    return ctx.base_update()


async def _handle_ask_user(ctx: _ToolCallContext) -> dict[str, Any]:
    # Only the runtime-owned intake node may ask a bound symptom intake; a
    # model-authored ask_user never carries purpose or state binding.
    arguments = {
        key: value
        for key, value in ctx.arguments.items()
        if key not in {"purpose", "state_binding"}
    }
    result = await ctx.executor.execute(ctx.tool_call_id, ctx.tool_name, arguments)
    if result.status != ToolStatus.INTERRUPTED:
        ctx.emit_tool_result({"status": "error", "error": result.error})
        return {"pending_tool_calls": ctx.remaining}

    answer = interrupt(
        {
            "interrupt_type": "ask_user",
            "tool_call_id": ctx.tool_call_id,
            "tool_name": ctx.tool_name,
            "question": result.content or {},
        }
    )
    ctx.emit_tool_result({"answer": answer})
    ctx.append_tool_message(json.dumps(answer, ensure_ascii=False))
    return ctx.base_update()


_TOOL_HANDLERS: dict[str, Callable[[_ToolCallContext], Awaitable[dict[str, Any]]]] = {
    "extract_symptom_info": _handle_extract_symptom_info,
    "record_lifestyle_context": _handle_record_lifestyle_context,
    "search_knowledge": _handle_search_knowledge,
    "record_answer_attribution": _handle_record_answer_attribution,
    "get_posture_analysis": _handle_get_posture_analysis,
    "ask_user": _handle_ask_user,
}

# Tool names the v3 manifest reserves for the source-validated intake node.
_V3_INTAKE_OWNED_TOOLS = {
    "extract_symptom_info",
    "record_lifestyle_context",
    "record_body_state",
}


async def execute_tool(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    pending_tool_calls = list(state.get("pending_tool_calls", []))
    if not pending_tool_calls:
        return {}

    tool_call = pending_tool_calls[0]
    remaining = pending_tool_calls[1:]
    tool_call_id = tool_call.get("id", "")
    tool_name = tool_call.get("name", "")
    arguments = tool_call.get("arguments", {}) or {}

    writer({"type": "tool_call", "id": tool_call_id, "tool": tool_name, "args": arguments})

    ctx = _ToolCallContext(
        state=state,
        writer=writer,
        executor=get_consultation_executor(),
        tool_call_id=tool_call_id,
        tool_name=tool_name,
        arguments=arguments,
        runtime_messages=list(state.get("runtime_messages", [])),
        remaining=remaining,
    )

    manifest = state.get("consultation_manifest") or get_consultation_manifest(None)
    if (
        manifest.tool_policy_revision == "consultation-tools-v3"
        and tool_name in _V3_INTAKE_OWNED_TOOLS
    ):
        # A hallucinated legacy tool name cannot bypass the v3 extraction gate.
        ctx.emit_tool_result(
            {
                "status": "failed",
                "error": "statement capture is owned by the source-validated intake node",
            }
        )
        ctx.append_tool_message("未保存：此工具不可用于重复或绕过原话核验。")
        return ctx.base_update()

    handler = _TOOL_HANDLERS.get(tool_name)
    if handler is not None:
        return await handler(ctx)

    ctx.emit_tool_result({"status": "error", "error": f"Unsupported tool: {tool_name}"})
    return {"pending_tool_calls": remaining}
