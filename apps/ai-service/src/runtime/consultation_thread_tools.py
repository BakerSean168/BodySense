"""Execute one model-requested consultation tool call."""

from __future__ import annotations

import json
from typing import Any

from langgraph.types import StreamWriter, interrupt

from ..services.agent.answer_attribution import (
    build_published_evidence_binding,
    validate_and_evaluate_attribution,
)
from ..services.agent.consultation_tools import get_consultation_executor
from ..services.agent.reply_fallback import emit_citation_events
from ..services.agent.tool_types import ToolStatus
from .consultation_thread_manifest import get_consultation_manifest
from .consultation_thread_state import ConsultationThreadState, _merge_symptoms


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

    executor = get_consultation_executor()
    runtime_messages = list(state.get("runtime_messages", []))

    manifest = state.get("consultation_manifest") or get_consultation_manifest(None)
    if manifest.tool_policy_revision == "consultation-tools-v3" and tool_name in {
        "extract_symptom_info",
        "record_lifestyle_context",
        "record_body_state",
    }:
        # A hallucinated legacy tool name cannot bypass the v3 extraction gate.
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {
                    "status": "failed",
                    "error": "statement capture is owned by the source-validated intake node",
                },
            }
        )
        runtime_messages.append(
            {
                "role": "tool",
                "tool_call_id": tool_call_id,
                "content": "未保存：此工具不可用于重复或绕过原话核验。",
            }
        )
        return {"runtime_messages": runtime_messages, "pending_tool_calls": remaining}

    if tool_name == "extract_symptom_info":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        normalized = result.content if result.status == ToolStatus.SUCCESS else arguments
        assert isinstance(normalized, dict), "extract_symptom_info must return a dict"
        writer({"type": "extracted_info", "info": normalized})
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"status": "ok"},
            }
        )
        runtime_messages.append(
            {"role": "tool", "tool_call_id": tool_call_id, "content": "症状信息已提取。"}
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
            "extracted_symptoms": _merge_symptoms(
                state.get("extracted_symptoms", []),
                [normalized],
            ),
        }

    if tool_name == "record_lifestyle_context":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        normalized = result.content if result.status == ToolStatus.SUCCESS else arguments
        assert isinstance(normalized, dict), "record_lifestyle_context must return a dict"
        writer({"type": "lifestyle_context", "context": normalized})
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"status": "ok"},
            }
        )
        runtime_messages.append(
            {"role": "tool", "tool_call_id": tool_call_id, "content": "生活方式信息已记录。"}
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
        }

    if tool_name == "search_knowledge":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        has_results = False
        retrieved = dict(state.get("retrieved_published_evidence", {}) or {})
        result_text = result.error or "搜索失败"
        if result.status == ToolStatus.SUCCESS:
            content = result.content if isinstance(result.content, dict) else {}
            result_text = str(content.get("result_text", ""))
            has_results = bool(content.get("has_results", False))
            raw_results = content.get("raw_results", [])
            if has_results:
                emit_citation_events(raw_results, writer)
                for raw_result in raw_results:
                    binding = build_published_evidence_binding(raw_result)
                    if binding is not None:
                        retrieved[str(binding["evidence_ref"])] = binding
            else:
                writer(
                    {
                        "type": "knowledge_gap",
                        "query": arguments.get("query", ""),
                        "message": "知识库中暂未找到相关专项资料，以下为通用建议。",
                    }
                )
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"has_results": has_results},
            }
        )
        runtime_messages.append(
            {"role": "tool", "tool_call_id": tool_call_id, "content": result_text}
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
            "retrieved_published_evidence": retrieved,
        }

    if tool_name == "record_answer_attribution":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        if result.status != ToolStatus.SUCCESS:
            writer(
                {
                    "type": "tool_result",
                    "id": tool_call_id,
                    "tool": tool_name,
                    "result": {"status": "error", "error": result.error or "invalid attribution"},
                }
            )
            runtime_messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call_id,
                    "content": result.error or "归因参数无效，请修正后重试。",
                }
            )
            return {"runtime_messages": runtime_messages, "pending_tool_calls": remaining}

        try:
            evaluated = validate_and_evaluate_attribution(
                list(arguments.get("claims") or []),
                dict(state.get("retrieved_published_evidence", {}) or {}),
            )
        except ValueError as exc:
            error_text = str(exc)
            writer(
                {
                    "type": "tool_result",
                    "id": tool_call_id,
                    "tool": tool_name,
                    "result": {"status": "error", "error": error_text},
                }
            )
            runtime_messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call_id,
                    "content": (
                        f"归因校验失败：{error_text}。"
                        "只能使用本轮搜索返回的 Published Evidence Ref。"
                    ),
                }
            )
            return {"runtime_messages": runtime_messages, "pending_tool_calls": remaining}

        existing_attributions = list(state.get("answer_attributions", []) or [])
        emitted: list[dict[str, Any]] = []
        for index, attribution in enumerate(evaluated):
            payload = {
                **attribution,
                "attribution_id": f"{tool_call_id}:{index}",
            }
            writer({"type": "answer_attribution", "attribution": payload})
            emitted.append(payload)
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"status": "ok", "recorded_claims": len(emitted)},
            }
        )
        runtime_messages.append(
            {
                "role": "tool",
                "tool_call_id": tool_call_id,
                "content": "回答证据归因已记录。现在直接给出自然语言回答，不要展示 Evidence Ref。",
            }
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
            "answer_attributions": existing_attributions + emitted,
        }

    if tool_name == "get_posture_analysis":
        # Inject prefetched analysis so the pure tool never needs a reverse HTTP call.
        tool_args = dict(arguments)
        tool_args["_posture_analysis"] = state.get("posture_analysis") or {
            "has_analysis": False,
            "views": [],
            "findings": [],
            "summaries": [],
        }
        result = await executor.execute(tool_call_id, tool_name, tool_args)
        has_analysis = False
        result_text = result.error or "读取体态分析失败"
        summary: dict[str, Any] = {}
        if result.status == ToolStatus.SUCCESS:
            content = result.content if isinstance(result.content, dict) else {}
            if isinstance(content, dict):
                result_text = str(content.get("result_text", ""))
                has_analysis = bool(content.get("has_analysis", False))
                summary = content.get("summary") or {}
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {
                    "has_analysis": has_analysis,
                    "summary": summary,
                },
            }
        )
        runtime_messages.append(
            {"role": "tool", "tool_call_id": tool_call_id, "content": result_text}
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
        }

    if tool_name == "ask_user":
        # Only the runtime-owned intake node may ask a bound symptom intake; a
        # model-authored ask_user never carries purpose or state binding.
        arguments = {
            key: value
            for key, value in arguments.items()
            if key not in {"purpose", "state_binding"}
        }
        result = await executor.execute(tool_call_id, tool_name, arguments)
        if result.status != ToolStatus.INTERRUPTED:
            writer(
                {
                    "type": "tool_result",
                    "id": tool_call_id,
                    "tool": tool_name,
                    "result": {"status": "error", "error": result.error},
                }
            )
            return {"pending_tool_calls": remaining}

        answer = interrupt(
            {
                "interrupt_type": "ask_user",
                "tool_call_id": tool_call_id,
                "tool_name": tool_name,
                "question": result.content or {},
            }
        )
        writer(
            {
                "type": "tool_result",
                "id": tool_call_id,
                "tool": tool_name,
                "result": {"answer": answer},
            }
        )
        runtime_messages.append(
            {
                "role": "tool",
                "tool_call_id": tool_call_id,
                "content": json.dumps(answer, ensure_ascii=False),
            }
        )
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": remaining,
        }

    writer(
        {
            "type": "tool_result",
            "id": tool_call_id,
            "tool": tool_name,
            "result": {"status": "error", "error": f"Unsupported tool: {tool_name}"},
        }
    )
    return {"pending_tool_calls": remaining}
