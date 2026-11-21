"""Checkpointed consultation thread runtime built on LangGraph."""

from __future__ import annotations

import asyncio
import logging
from collections.abc import AsyncIterator
from typing import Any, assert_never, cast
from uuid import UUID

from langchain_core.runnables import RunnableConfig
from langgraph.types import Command, StreamWriter

from ..ai import AiRequest, AIService, GatewayUnavailableError
from ..ai.consultation_gateway_model import consultation_model_settings
from ..ai.types import AiDoneEvent, AiTextDeltaEvent, AiToolCallDoneEvent, AiUsageEvent
from ..models.body_fact_intake import BODY_FACT_INTAKE_POLICY
from ..models.consultation_runtime_event import (
    ConsultationRuntimeEvent,
    ConsultationRuntimeEventFactory,
    InteractionRequiredRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamDoneRuntimeEvent,
)
from ..services.agent.consultation_tools import get_consultation_registry
from ..services.agent.reply_fallback import build_fallback_reply
from ..services.red_flag_detector import get_red_flag_detector
from .body_fact_capture import acquire_body_fact_answer
from .checkpointing import get_runtime_checkpointer
from .consultation_thread_context import (
    _format_longitudinal_context,
    _format_spatial_context,
)
from .consultation_thread_events import _map_internal_event, _runtime_agent_configuration_event
from .consultation_thread_graph import _build_graph
from .consultation_thread_guard import (
    STREAM_TAIL_HOLD_CHARS,
    _chunk_text,
    _guard_final_assistant_text,
)
from .consultation_thread_manifest import (
    _checkpoint_manifest_configuration_id,
    get_consultation_manifest,
)
from .consultation_thread_prompt import (
    MAX_CONTEXT_TURNS,
    _estimate_tool_overhead_tokens,
    _get_conversation_text,
    _runtime_messages_to_chat_messages,
    context_history,
)
from .consultation_thread_state import (
    ConsultationThreadState,
    _user_content_with_images,
)
from .consultation_thread_tools import execute_tool
from .context_budget import ContextBudgetError

__all__ = [
    "MAX_CONTEXT_TURNS",
    "MAX_TOOL_ROUNDS",
    "ConsultationThreadState",
    "_build_graph",
    "_format_longitudinal_context",
    "_format_spatial_context",
    "_get_conversation_text",
    "_guard_final_assistant_text",
    "_map_internal_event",
    "_runtime_messages_to_chat_messages",
    "_user_content_with_images",
    "context_history",
    "execute_tool",
    "get_consultation_manifest",
    "get_runtime_graph",
    "llm_turn",
    "resume_thread_interrupt",
    "stream_thread_turn",
]

logger = logging.getLogger(__name__)

MAX_TOOL_ROUNDS = 6

_ai_service_instance: AIService | None = None
_compiled_graph = None


def _get_ai_service() -> AIService:
    global _ai_service_instance
    if _ai_service_instance is None:
        _ai_service_instance = AIService()
    return _ai_service_instance


async def llm_turn(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    tool_rounds = state.get("tool_rounds", 0)
    if tool_rounds >= MAX_TOOL_ROUNDS:
        writer({"type": "stream_error", "message": "模型工具循环超过上限"})
        return {"pending_tool_calls": [], "tool_rounds": tool_rounds}

    # North-Star: resolve the exact immutable manifest for this turn.
    manifest = state.get("consultation_manifest") or get_consultation_manifest(None)

    try:
        ai = _get_ai_service()
    except GatewayUnavailableError:
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
    if manifest.tool_policy_revision == "consultation-tools-v3":
        provider_tools = [
            tool
            for tool in provider_tools
            if tool.name not in {"extract_symptom_info", "record_lifestyle_context"}
        ]
    raw_text = ""
    emitted_text_length = 0
    completed_tool_calls: list[dict[str, Any]] = []

    # #494: an unfittable request fails predictably here (explicit stream
    # error) instead of overflowing the model window mid-generation.
    try:
        model_messages = _runtime_messages_to_chat_messages(
            state,
            tool_overhead_tokens=_estimate_tool_overhead_tokens(provider_tools),
            max_output_tokens=manifest.generation.max_tokens,
        )
    except ContextBudgetError as exc:
        writer({"type": "stream_error", "message": f"上下文超出模型窗口预算：{exc}"})
        return {"pending_tool_calls": [], "tool_rounds": tool_rounds}

    # North-Star: pin the exact logical model + generation settings from the
    # immutable manifest so the runtime honors the exact configuration identity.
    async for event in ai.generate_stream(
        AiRequest(
            use_case="consultation.reply",
            messages=model_messages,
            tools=provider_tools,
            temperature=manifest.generation.temperature,
            max_tokens=manifest.generation.max_tokens,
            logical_model=manifest.logical_model,
            model_settings=consultation_model_settings(manifest),
        )
    ):
        if isinstance(event, AiTextDeltaEvent):
            if not event.text:
                continue
            raw_text += event.text
            safe_end = max(0, len(raw_text) - STREAM_TAIL_HOLD_CHARS)
            if safe_end > emitted_text_length:
                delta = raw_text[emitted_text_length:safe_end]
                emitted_text_length = safe_end
                writer({"type": "text_delta", "delta": delta})
                await asyncio.sleep(0)
        elif isinstance(event, AiToolCallDoneEvent):
            if not any(existing["id"] == event.tool_call_id for existing in completed_tool_calls):
                completed_tool_calls.append(
                    {
                        "id": event.tool_call_id,
                        "name": event.tool_name,
                        "arguments": dict(event.tool_arguments),
                    }
                )
        elif isinstance(event, AiUsageEvent):
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
        elif isinstance(event, AiDoneEvent):
            continue
        else:
            assert_never(event)

    accumulated_text, guarded_question = _guard_final_assistant_text(raw_text)
    # Do not synthesize a question on an intermediate tool round; the model gets
    # another turn after tool results and can decide again with the new evidence.
    if completed_tool_calls:
        guarded_question = None
    elif guarded_question:
        completed_tool_calls.append(
            {
                "id": f"guard-ask-{state.get('run_id', 'run')}-{tool_rounds}",
                "name": "ask_user",
                "arguments": {
                    "question": guarded_question,
                    "answer_type": "text",
                    "allow_custom_input": True,
                },
            }
        )

    if len(accumulated_text) < emitted_text_length:
        # The guard only owns the held tail. If a provider emits an exceptionally
        # long final question, never pretend already-streamed bytes were retracted.
        accumulated_text = raw_text
        guarded_question = None
        completed_tool_calls = [
            call
            for call in completed_tool_calls
            if not str(call.get("id", "")).startswith("guard-ask-")
        ]
    tail = accumulated_text[emitted_text_length:]
    if tail:
        writer({"type": "text_delta", "delta": tail})
        await asyncio.sleep(0)

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


async def get_runtime_graph():
    global _compiled_graph
    if _compiled_graph is None:
        _compiled_graph = _build_graph(await get_runtime_checkpointer())
    return _compiled_graph


async def stream_thread_turn(
    *,
    thread_id: str,
    conversation_id: str,
    run_id: str,
    user_id: str,
    user_message: str,
    user_message_id: str | None = None,
    images: list[dict[str, Any]] | None = None,
    profile: dict[str, Any],
    extracted_info: list[dict[str, Any]],
    phase: str,
    body_state: dict[str, Any] | None = None,
    posture_analysis: dict[str, Any] | None = None,
    relevant_history: list[dict[str, Any]] | None = None,
    episodic_notes: list[dict[str, Any]] | None = None,
    current_diagnosis: dict[str, Any] | None = None,
    current_treatment: dict[str, Any] | None = None,
    recent_outcomes: list[dict[str, Any]] | None = None,
    spatial_context: dict[str, Any] | None = None,
    configuration_id: str | None = None,
    context_window: dict[str, Any] | None = None,
) -> AsyncIterator[ConsultationRuntimeEvent]:
    graph = await get_runtime_graph()
    config = cast(RunnableConfig, {"configurable": {"thread_id": thread_id}})
    factory = ConsultationRuntimeEventFactory(conversation_id=conversation_id)

    # Resolve the exact immutable Agent configuration (North-Star identity).
    manifest = get_consultation_manifest(configuration_id)

    # Control-plane handshake MUST be first. Go validates and durably records
    # this exact identity before it accepts any semantic/tool/state output.
    yield _runtime_agent_configuration_event(factory, manifest=manifest, run_id=run_id)

    async for chunk in graph.astream(
        {
            "session_id": conversation_id,
            "run_id": run_id,
            "user_id": user_id,
            "profile": profile,
            "body_state": body_state or {},
            "relevant_history": list(relevant_history or []),
            "episodic_notes": list(episodic_notes or []),
            "current_diagnosis": current_diagnosis or {},
            "current_treatment": current_treatment or {},
            "recent_outcomes": list(recent_outcomes or []),
            "spatial_context": spatial_context or {},
            # Set every turn so a window from an earlier turn never lingers.
            "context_window": context_window,
            "phase": phase,
            "extracted_symptoms": extracted_info,
            "current_user_message": user_message,
            "current_user_message_id": user_message_id or "",
            "pending_user_images": list(images or []),
            "posture_analysis": posture_analysis
            or {
                "has_analysis": False,
                "views": [],
                "findings": [],
                "summaries": [],
            },
            "consultation_manifest": manifest,
        },
        config=config,
        stream_mode="custom",
    ):
        event = _map_internal_event(factory, chunk, run_id=run_id)
        if event is not None:
            yield event
    snapshot = await graph.aget_state(config)
    if snapshot.interrupts:
        pending_interrupt = snapshot.interrupts[0]
        payload = pending_interrupt.value
        event = factory.next(
            InteractionRequiredRuntimeEvent(
                interaction_id=pending_interrupt.id,
                question=payload.get("question", {}),
                suggestion=payload.get("suggestion"),
            ),
            run_id=run_id,
            tool_call_id=payload.get("tool_call_id") or None,
        )
        yield event
        return

    yield factory.next(StreamDoneRuntimeEvent(), run_id=run_id)


async def resume_thread_interrupt(
    *,
    thread_id: str,
    conversation_id: str,
    run_id: str,
    configuration_id: str,
    answer: dict[str, Any],
    interrupt_id: str | None = None,
    profile: dict[str, Any] | None = None,
    body_state: dict[str, Any] | None = None,
    relevant_history: list[dict[str, Any]] | None = None,
    episodic_notes: list[dict[str, Any]] | None = None,
    current_diagnosis: dict[str, Any] | None = None,
    current_treatment: dict[str, Any] | None = None,
    recent_outcomes: list[dict[str, Any]] | None = None,
    spatial_context: dict[str, Any] | None = None,
    context_window: dict[str, Any] | None = None,
) -> AsyncIterator[ConsultationRuntimeEvent]:
    graph = await get_runtime_graph()
    config = cast(RunnableConfig, {"configurable": {"thread_id": thread_id}})
    factory = ConsultationRuntimeEventFactory(conversation_id=conversation_id)

    # A HITL resume is continuation of the same logical LangGraph thread. The
    # Go source run supplies its durable configuration id; reconcile that with
    # the checkpoint before executing any resumed semantic work.
    manifest = get_consultation_manifest(configuration_id)
    checkpoint = await graph.aget_state(config)
    checkpoint_values = getattr(checkpoint, "values", None)
    if not isinstance(checkpoint_values, dict):
        raise ValueError("Consultation checkpoint has no runtime state")
    checkpoint_configuration_id = _checkpoint_manifest_configuration_id(
        checkpoint_values.get("consultation_manifest")
    )
    if checkpoint_configuration_id != manifest.configuration_id:
        raise ValueError(
            "Consultation resume configuration mismatch: "
            f"checkpoint={checkpoint_configuration_id} requested={manifest.configuration_id}"
        )

    body_fact_resume = bool(
        manifest.intake and manifest.intake.policy_revision == BODY_FACT_INTAKE_POLICY
    )
    safety_screen_resume = False
    if body_fact_resume:
        if not interrupt_id or checkpoint_values.get("run_id") != run_id:
            raise ValueError(
                "BodyFact resume must preserve its original Run and interrupt identity"
            )
        pending = getattr(checkpoint, "interrupts", ())
        if len(pending) != 1 or UUID(str(pending[0].id)).hex != UUID(interrupt_id).hex:
            raise ValueError("Resume does not match the pending native interrupt")
        pending_value = getattr(pending[0], "value", None)
        pending_question = (
            pending_value.get("question") if isinstance(pending_value, dict) else None
        )
        safety_screen_resume = (
            isinstance(pending_question, dict)
            and pending_question.get("purpose") == "symptom_intake"
        )

    yield _runtime_agent_configuration_event(factory, manifest=manifest, run_id=run_id)
    answer_safety: dict[str, Any] | None = None
    if body_fact_resume:
        # The original safety node ran before the interruption. New user input
        # must pass the same existing screen before extraction or advice; a
        # negative screen never clears an earlier independent restriction.
        detected = get_red_flag_detector().detect([], str(answer.get("text", "")))
        if detected.has_red_flags:
            answer_safety = detected.to_dict()
            yield factory.next(
                RedFlagDetectedRuntimeEvent(
                    has_red_flags=True, flags=answer_safety.get("flags", [])
                ),
                run_id=run_id,
            )
    answer_intake: dict[str, Any] | None = None
    # A safety-screen checklist answer is recorded by Go as the screen of the
    # already committed symptom; it is not a free-text self statement, so it
    # never goes through statement extraction (the red-flag screen above
    # still ran on it).
    if body_fact_resume and interrupt_id and not safety_screen_resume:
        capture_events: list[dict[str, Any]] = []
        answer_intake = await acquire_body_fact_answer(
            run_id=run_id,
            interrupt_id=interrupt_id,
            answer=answer,
            body_state=body_state or {},
            writer=capture_events.append,
            manifest=manifest,
        )
        for capture_event in capture_events:
            mapped = _map_internal_event(factory, capture_event, run_id=run_id)
            if mapped is not None:
                yield mapped

    # Refresh durable business context at resume time as well. The LangGraph
    # checkpoint owns runtime protocol state, while Go-owned BodyState may have
    # changed because the user answered/edited structured health information.
    async for chunk in graph.astream(
        Command(
            resume=answer,
            update={
                **({"intake_result": answer_intake} if answer_intake is not None else {}),
                # body_state below is fresh from Go, so only this resume's own
                # answer facts are still pending as revisions.
                "body_state_pending_revisions": (
                    1 if (answer_intake or {}).get("capture_status") == "proposed" else 0
                ),
                **({"red_flag_result": answer_safety} if answer_safety is not None else {}),
                "profile": profile or {},
                "body_state": body_state or {},
                "relevant_history": list(relevant_history or []),
                "episodic_notes": list(episodic_notes or []),
                "current_diagnosis": current_diagnosis or {},
                "current_treatment": current_treatment or {},
                "recent_outcomes": list(recent_outcomes or []),
                "spatial_context": spatial_context or {},
                "context_window": context_window,
                # One answered HITL interrupt ends the previous autonomous
                # LLM/tool segment. Keep the unattended MAX_TOOL_ROUNDS guard,
                # but never count earlier user answers against its budget.
                "tool_rounds": 0,
            },
        ),
        config=config,
        stream_mode="custom",
    ):
        event = _map_internal_event(factory, chunk, run_id=run_id)
        if event is not None:
            yield event

    snapshot = await graph.aget_state(config)
    if snapshot.interrupts:
        pending_interrupt = snapshot.interrupts[0]
        payload = pending_interrupt.value
        event = factory.next(
            InteractionRequiredRuntimeEvent(
                interaction_id=pending_interrupt.id,
                question=payload.get("question", {}),
                suggestion=payload.get("suggestion"),
            ),
            run_id=run_id,
            tool_call_id=payload.get("tool_call_id") or None,
        )
        yield event
        return

    yield factory.next(StreamDoneRuntimeEvent(), run_id=run_id)
