"""Checkpointed consultation thread runtime built on LangGraph."""

from __future__ import annotations

import asyncio
import json
import logging
from collections.abc import AsyncIterator
from typing import Annotated, Any, Literal, TypedDict

from langgraph.graph import END, START, StateGraph
from langgraph.types import Command, StreamWriter, interrupt

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage, ToolCall
from ..models.consultation import ChatContext, ExtractedInfo
from ..models.stream_event import StreamEvent, StreamEventFactory, StreamEventIds
from ..prompts.consultation import format_profile_context, get_system_prompt
from ..services.agent.consultation_tools import (
    get_consultation_executor,
    get_consultation_registry,
)
from ..services.agent.orchestrator import build_fallback_reply, emit_citation_events
from ..services.agent.tool_types import ToolStatus
from ..services.agent_workflow import get_agent_workflow
from ..services.red_flag_detector import get_red_flag_detector
from .checkpointing import get_runtime_checkpointer

logger = logging.getLogger(__name__)

MAX_CONTEXT_TURNS = 10
MAX_TOOL_ROUNDS = 6


def _merge_symptoms(
    existing: list[dict[str, Any]],
    new: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    by_part: dict[str, dict[str, Any]] = {}
    for symptom in existing:
        part = symptom.get("body_part", "")
        if part:
            by_part[part] = dict(symptom)
    for symptom in new:
        part = symptom.get("body_part", "")
        if not part:
            continue
        if part in by_part:
            by_part[part].update(symptom)
        else:
            by_part[part] = dict(symptom)
    return list(by_part.values())


def _health_features_from_symptom(info: dict[str, Any]) -> dict[str, Any]:
    body_part = str(info.get("body_part", "") or "").strip()
    label = str(info.get("symptom_type", "") or "").strip() or body_part
    if not label:
        return {
            "posture_findings": [],
            "discomforts": [],
            "negative_findings": [],
            "movement_limitations": [],
            "red_flags": [],
            "user_answers": [],
        }

    item: dict[str, Any] = {
        "label": label,
        "source": "extracted_info",
    }
    if body_part:
        item["body_part"] = body_part
    severity = str(info.get("severity", "") or "").strip()
    if severity:
        item["value"] = severity

    details = [
        str(info.get(key, "") or "").strip()
        for key in ("duration", "trigger", "relief")
        if str(info.get(key, "") or "").strip()
    ]
    if details:
        item["details"] = "，".join(details)

    return {
        "posture_findings": [],
        "discomforts": [item],
        "negative_findings": [],
        "movement_limitations": [],
        "red_flags": [],
        "user_answers": [],
    }


class ConsultationThreadState(TypedDict, total=False):
    session_id: str
    user_id: str
    profile: dict[str, Any]
    phase: str
    current_user_message: str
    runtime_messages: list[dict[str, Any]]
    extracted_symptoms: Annotated[list[dict[str, Any]], _merge_symptoms]
    red_flag_result: dict[str, Any] | None
    intent: str
    workflow_action: str
    accumulated_text: str
    pending_tool_calls: list[dict[str, Any]]
    tool_rounds: int
    llm_available: bool
    diagnosis_result: dict[str, Any] | None
    treatment_result: dict[str, Any] | None


_ai_service_instance: AIService | None = None
_compiled_graph = None


def _get_ai_service() -> AIService:
    global _ai_service_instance
    if _ai_service_instance is None:
        _ai_service_instance = AIService()
    return _ai_service_instance


def _runtime_messages_to_chat_messages(state: ConsultationThreadState) -> list[ChatMessage]:
    messages: list[ChatMessage] = []

    profile_context = format_profile_context(state.get("profile", {}))
    system_content = get_system_prompt(profile_context)

    extracted = state.get("extracted_symptoms", [])
    if extracted:
        info_lines = ["## 已提取的症状信息"]
        for symptom in extracted:
            body_part = symptom.get("body_part", "")
            if not body_part:
                continue
            line = f"- {body_part}：{symptom.get('symptom_type', '待补充')}"
            if symptom.get("duration"):
                line += f"，持续{symptom['duration']}"
            if symptom.get("trigger"):
                line += f"，{symptom['trigger']}时出现"
            info_lines.append(line)
        if len(info_lines) > 1:
            system_content += "\n\n" + "\n".join(info_lines)

    messages.append(ChatMessage(role="system", content=system_content))

    history = state.get("runtime_messages", [])
    history = history[-MAX_CONTEXT_TURNS * 4 :]
    for item in history:
        chat_message = ChatMessage(
            role=item["role"],
            content=item.get("content", ""),
        )
        tool_calls = item.get("tool_calls") or []
        if tool_calls:
            chat_message.tool_calls = [
                ToolCall(
                    id=tool_call["id"],
                    name=tool_call["name"],
                    arguments=tool_call.get("arguments", {}),
                )
                for tool_call in tool_calls
            ]
        tool_call_id = item.get("tool_call_id")
        if tool_call_id:
            chat_message.tool_call_id = tool_call_id
        messages.append(chat_message)

    return messages


def _get_conversation_text(state: ConsultationThreadState) -> str:
    texts: list[str] = []
    for message in state.get("runtime_messages", [])[-MAX_CONTEXT_TURNS * 4 :]:
        content = message.get("content", "")
        if isinstance(content, str) and content:
            texts.append(content)
    return " ".join(texts)


def _determine_phase(extracted_symptoms: list[dict[str, Any]]) -> str:
    for symptom in extracted_symptoms:
        body_part = symptom.get("body_part", "")
        if not body_part:
            continue
        has_detail = any(
            symptom.get(key) for key in ("symptom_type", "duration", "trigger", "severity")
        )
        if has_detail:
            return "ready_for_analysis"
    return "collecting"


async def prepare_turn(state: ConsultationThreadState) -> dict[str, Any]:
    current_user_message = state.get("current_user_message", "").strip()
    if not current_user_message:
        return {}

    runtime_messages = list(state.get("runtime_messages", []))
    runtime_messages.append({"role": "user", "content": current_user_message})
    return {
        "runtime_messages": runtime_messages,
        "current_user_message": "",
        "pending_tool_calls": [],
        "accumulated_text": "",
        "tool_rounds": 0,
    }


async def safety_check(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    detector = get_red_flag_detector()
    red_flag_result = detector.detect(
        state.get("extracted_symptoms", []),
        _get_conversation_text(state),
    )

    if red_flag_result.has_red_flags:
        writer({"type": "red_flag", **red_flag_result.to_dict()})

    return {
        "red_flag_result": red_flag_result.to_dict() if red_flag_result.has_red_flags else None,
    }


async def classify_intent(state: ConsultationThreadState) -> dict[str, Any]:
    workflow = get_agent_workflow()
    context = ChatContext(
        session_id=state.get("session_id", ""),
        user_id=state.get("user_id", ""),
        profile=state.get("profile", {}),
        extracted_info=ExtractedInfo.from_dict(state.get("extracted_symptoms", [])),
        messages=state.get("runtime_messages", []),
        phase=state.get("phase", "collecting"),
    )

    user_message = ""
    for message in reversed(state.get("runtime_messages", [])):
        if message.get("role") == "user":
            user_message = str(message.get("content", ""))
            break

    intent = workflow.classify_intent(user_message, context)
    decision = workflow.decide_next_action(intent, context, user_message)
    return {
        "intent": intent.value,
        "workflow_action": decision.action.value,
    }


async def llm_turn(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    tool_rounds = state.get("tool_rounds", 0)
    if tool_rounds >= MAX_TOOL_ROUNDS:
        writer({"type": "stream_error", "message": "模型工具循环超过上限"})
        return {"pending_tool_calls": [], "tool_rounds": tool_rounds}

    try:
        ai = _get_ai_service()
    except Exception:
        user_message = ""
        for message in reversed(state.get("runtime_messages", [])):
            if message.get("role") == "user":
                user_message = str(message.get("content", ""))
                break
        fallback_text = build_fallback_reply(user_message, [])
        for chunk in _chunk_text(fallback_text):
            writer({"type": "text_delta", "delta": chunk})
            await asyncio.sleep(0)
        runtime_messages = list(state.get("runtime_messages", []))
        runtime_messages.append({"role": "assistant", "content": fallback_text})
        return {
            "runtime_messages": runtime_messages,
            "pending_tool_calls": [],
            "accumulated_text": fallback_text,
            "tool_rounds": tool_rounds + 1,
            "llm_available": False,
        }

    provider_tools = get_consultation_registry().to_provider_tools()
    accumulated_text = ""
    completed_tool_calls: list[dict[str, Any]] = []

    async for event in ai.generate_stream(
        AiRequest(
            use_case="consultation.reply",
            messages=_runtime_messages_to_chat_messages(state),
            tools=provider_tools,
            temperature=0.7,
            max_tokens=2048,
        )
    ):
        if event.type == "text_delta" and event.text:
            accumulated_text += event.text
            writer({"type": "text_delta", "delta": event.text})
            await asyncio.sleep(0)
        elif event.type == "tool_call_done" and event.tool_name:
            tool_call_id = event.tool_call_id or ""
            if not any(existing["id"] == tool_call_id for existing in completed_tool_calls):
                completed_tool_calls.append(
                    {
                        "id": tool_call_id,
                        "name": event.tool_name,
                        "arguments": event.tool_arguments or {},
                    }
                )
        elif event.type == "usage" and event.usage:
            writer(
                {
                    "type": "usage",
                    "usage": {
                        "input_tokens": event.usage.input_tokens,
                        "output_tokens": event.usage.output_tokens,
                        "total_tokens": event.usage.total_tokens,
                    },
                }
            )

    runtime_messages = list(state.get("runtime_messages", []))
    assistant_message: dict[str, Any] = {
        "role": "assistant",
        "content": accumulated_text,
    }
    if completed_tool_calls:
        assistant_message["tool_calls"] = completed_tool_calls
    runtime_messages.append(assistant_message)

    return {
        "runtime_messages": runtime_messages,
        "pending_tool_calls": completed_tool_calls,
        "accumulated_text": accumulated_text,
        "tool_rounds": tool_rounds + 1,
        "llm_available": True,
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

    executor = get_consultation_executor()
    runtime_messages = list(state.get("runtime_messages", []))

    if tool_name == "extract_symptom_info":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        normalized = result.content if result.status == ToolStatus.SUCCESS else arguments
        writer({"type": "extracted_info", "info": normalized})
        writer(
            {
                "type": "health_features",
                "health_features": _health_features_from_symptom(normalized),
            }
        )
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

    if tool_name == "search_knowledge":
        result = await executor.execute(tool_call_id, tool_name, arguments)
        has_results = False
        result_text = result.error or "搜索失败"
        if result.status == ToolStatus.SUCCESS:
            content = result.content or {}
            result_text = str(content.get("result_text", ""))
            has_results = bool(content.get("has_results", False))
            raw_results = content.get("raw_results", [])
            if has_results:
                emit_citation_events(raw_results, writer)
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
        }

    if tool_name == "ask_user":
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


def route_after_model(state: ConsultationThreadState) -> Literal["execute_tool", "decide_phase"]:
    if state.get("pending_tool_calls"):
        return "execute_tool"
    return "decide_phase"


def route_after_tool(state: ConsultationThreadState) -> Literal["execute_tool", "llm_turn"]:
    if state.get("pending_tool_calls"):
        return "execute_tool"
    return "llm_turn"


async def decide_phase(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    extracted_symptoms = state.get("extracted_symptoms", [])
    new_phase = _determine_phase(extracted_symptoms)
    current_phase = state.get("phase", "collecting")
    if new_phase != current_phase:
        reason = (
            "symptom details collected"
            if new_phase == "ready_for_analysis"
            else "phase updated"
        )
        writer({"type": "phase_change", "phase": new_phase, "reason": reason})
    return {"phase": new_phase}


async def emit_done(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    writer(
        {
            "type": "__done__",
            "session_id": state.get("session_id", ""),
            "full_text": state.get("accumulated_text", ""),
            "extracted_info": state.get("extracted_symptoms", []),
            "phase": state.get("phase", "collecting"),
        }
    )
    return {}


def _build_graph(checkpointer: Any):
    graph = StateGraph(ConsultationThreadState)
    graph.add_node("prepare_turn", prepare_turn)
    graph.add_node("safety_check", safety_check)
    graph.add_node("classify_intent", classify_intent)
    graph.add_node("llm_turn", llm_turn)
    graph.add_node("execute_tool", execute_tool)
    graph.add_node("decide_phase", decide_phase)
    graph.add_node("emit_done", emit_done)

    graph.add_edge(START, "prepare_turn")
    graph.add_edge("prepare_turn", "safety_check")
    graph.add_edge("safety_check", "classify_intent")
    graph.add_edge("classify_intent", "llm_turn")
    graph.add_conditional_edges("llm_turn", route_after_model)
    graph.add_conditional_edges("execute_tool", route_after_tool)
    graph.add_edge("decide_phase", "emit_done")
    graph.add_edge("emit_done", END)

    return graph.compile(checkpointer=checkpointer)


async def get_runtime_graph():
    global _compiled_graph
    if _compiled_graph is None:
        _compiled_graph = _build_graph(await get_runtime_checkpointer())
    return _compiled_graph


def _map_internal_event(
    factory: StreamEventFactory,
    event_data: dict[str, Any],
) -> StreamEvent | None:
    event_type = event_data.get("type")
    if event_type == "text_delta":
        return factory.next(
            channel="message",
            event_type="message.text.delta",
            payload={"delta": event_data.get("delta", "")},
        )
    if event_type == "tool_call":
        return factory.next(
            channel="tool",
            event_type="tool.call",
            payload={
                "tool": event_data.get("tool", ""),
                "args": event_data.get("args", {}),
            },
            ids=StreamEventIds(tool_call_id=event_data.get("id") or None),
        )
    if event_type == "tool_result":
        return factory.next(
            channel="tool",
            event_type="tool.result",
            payload={
                "tool": event_data.get("tool", ""),
                "result": event_data.get("result", {}),
            },
            ids=StreamEventIds(tool_call_id=event_data.get("id") or None),
        )
    if event_type == "extracted_info":
        return factory.next(
            channel="state",
            event_type="state.extracted_info.upsert",
            payload={"info": event_data.get("info", {})},
        )
    if event_type == "phase_change":
        return factory.next(
            channel="state",
            event_type="state.phase.changed",
            payload={"to": event_data.get("phase", ""), "reason": event_data.get("reason", "")},
        )
    if event_type == "citation":
        return factory.next(
            channel="source",
            event_type="source.citation.added",
            payload={"citation": event_data.get("citation", {})},
        )
    if event_type == "knowledge_gap":
        return factory.next(
            channel="source",
            event_type="source.knowledge_gap",
            payload={
                "query": event_data.get("query", ""),
                "message": event_data.get("message", ""),
            },
        )
    if event_type == "red_flag":
        return factory.next(
            channel="safety",
            event_type="safety.red_flag.detected",
            payload={
                "has_red_flags": bool(event_data.get("has_red_flags", False)),
                "flags": event_data.get("flags", []),
            },
        )
    if event_type == "usage":
        return factory.next(
            channel="usage",
            event_type="usage.reported",
            payload={"usage": event_data.get("usage", {})},
        )
    return None


async def stream_thread_turn(
    *,
    thread_id: str,
    conversation_id: str,
    run_id: str,
    user_id: str,
    user_message: str,
    profile: dict[str, Any],
    extracted_info: list[dict[str, Any]],
    phase: str,
) -> AsyncIterator[StreamEvent]:
    graph = await get_runtime_graph()
    config = {"configurable": {"thread_id": thread_id}}
    factory = StreamEventFactory(conversation_id=conversation_id)

    async for chunk in graph.astream(
        {
            "session_id": conversation_id,
            "user_id": user_id,
            "profile": profile,
            "phase": phase,
            "extracted_symptoms": extracted_info,
            "current_user_message": user_message,
        },
        config=config,
        stream_mode="custom",
    ):
        event = _map_internal_event(factory, chunk)
        if event is not None:
            event.ids.run_id = run_id
            yield event

    snapshot = await graph.aget_state(config)
    if snapshot.interrupts:
        pending_interrupt = snapshot.interrupts[0]
        payload = pending_interrupt.value
        event = factory.next(
            channel="state",
            event_type="state.interaction.required",
            payload={
                "interaction_id": pending_interrupt.id,
                "question": payload.get("question", {}),
            },
            ids=StreamEventIds(
                run_id=run_id,
                tool_call_id=payload.get("tool_call_id") or None,
                interaction_id=pending_interrupt.id,
            ),
        )
        yield event
        return

    yield factory.next(
        channel="stream",
        event_type="stream.done",
        payload={},
        ids=StreamEventIds(run_id=run_id),
    )


async def resume_thread_interrupt(
    *,
    thread_id: str,
    conversation_id: str,
    run_id: str,
    answer: dict[str, Any],
) -> AsyncIterator[StreamEvent]:
    graph = await get_runtime_graph()
    config = {"configurable": {"thread_id": thread_id}}
    factory = StreamEventFactory(conversation_id=conversation_id)

    async for chunk in graph.astream(
        Command(resume=answer),
        config=config,
        stream_mode="custom",
    ):
        event = _map_internal_event(factory, chunk)
        if event is not None:
            event.ids.run_id = run_id
            yield event

    snapshot = await graph.aget_state(config)
    if snapshot.interrupts:
        pending_interrupt = snapshot.interrupts[0]
        payload = pending_interrupt.value
        event = factory.next(
            channel="state",
            event_type="state.interaction.required",
            payload={
                "interaction_id": pending_interrupt.id,
                "question": payload.get("question", {}),
            },
            ids=StreamEventIds(
                run_id=run_id,
                tool_call_id=payload.get("tool_call_id") or None,
                interaction_id=pending_interrupt.id,
            ),
        )
        yield event
        return

    yield factory.next(
        channel="stream",
        event_type="stream.done",
        payload={},
        ids=StreamEventIds(run_id=run_id),
    )


def _chunk_text(text: str, chunk_size: int = 120) -> list[str]:
    return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size)]
