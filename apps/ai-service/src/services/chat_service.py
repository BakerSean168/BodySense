"""Chat service for consultation conversations."""

import logging
import uuid
from collections.abc import AsyncIterator
from typing import Any

from ..models.consultation import ChatContext
from ..models.stream_event import StreamEvent, StreamEventFactory, StreamEventIds

logger = logging.getLogger(__name__)


class _StreamAccumulator:
    """Accumulates usage and text across the stream for the done event."""

    def __init__(self) -> None:
        self.total_input_tokens = 0
        self.total_output_tokens = 0
        self.full_text = ""


class ChatService:
    """Service for managing consultation chat conversations."""

    async def stream_chat(
        self,
        context: ChatContext,
        user_message: str,
        rag_results: list[dict[str, Any]] | None = None,
    ) -> AsyncIterator[StreamEvent]:
        """
        Process a user message and stream the AI response.

        Delegates to the LangGraph consultation graph for explicit workflow
        orchestration. Graph node events are normalized into StreamEvent
        envelopes before being yielded.

        Args:
            context: The chat session context.
            user_message: The user's message text.
            rag_results: Optional RAG search results for context.

        Yields:
            StreamEvent objects to send to the client.
        """
        from .consultation_graph import stream_consultation

        graph_state = {
            "session_id": context.session_id,
            "user_id": context.user_id,
            "user_message": user_message,
            "profile": context.profile,
            "conversation_history": context.messages,
            "rag_results": rag_results or [],
            "extracted_symptoms": context.extracted_info.to_dict(),
            "red_flag_result": None,
            "intent": "",
            "workflow_action": "",
            "accumulated_text": "",
            "phase": context.phase,
            "llm_available": True,
            "diagnosis_result": None,
            "treatment_result": None,
        }

        response_id = f"resp_{uuid.uuid4().hex[:12]}"
        acc = _StreamAccumulator()
        factory = StreamEventFactory(conversation_id=context.session_id)

        async for event_data in stream_consultation(graph_state):
            if event_data.get("type") == "__done__":
                done_payload = {
                    "response_id": response_id,
                    "usage": {
                        "input_tokens": acc.total_input_tokens,
                        "output_tokens": acc.total_output_tokens,
                    },
                    # Domain-specific extensions
                    "session_id": event_data.get("session_id", context.session_id),
                    "full_text": acc.full_text or event_data.get("full_text", ""),
                    "extracted_info": event_data.get("extracted_info", []),
                    "phase": event_data.get("phase", "collecting"),
                }

                # Run governance check (observe-only, non-blocking)
                try:
                    from .governance import AIOutputGuard
                    guard = AIOutputGuard()
                    gov_result = guard.validate_text_output(
                        acc.full_text or event_data.get("full_text", ""),
                        context={"extracted_info": event_data.get("extracted_info", [])},
                    )
                    done_payload["governance"] = gov_result.to_dict()
                except Exception:
                    logger.debug("Governance check failed (non-blocking)", exc_info=True)
                yield factory.next(
                    channel="stream",
                    event_type="stream.done",
                    payload=done_payload,
                )
            else:
                if event_data.get("type") == "text_delta":
                    acc.full_text += event_data.get("delta", "")
                elif event_data.get("type") == "usage":
                    usage = event_data.get("usage", {})
                    acc.total_input_tokens += usage.get("input_tokens", 0)
                    acc.total_output_tokens += usage.get("output_tokens", 0)
                yield _map_graph_event(factory, event_data)


def _map_graph_event(
    factory: StreamEventFactory,
    event_data: dict[str, Any],
) -> StreamEvent:
    """Map graph-internal events to the public StreamEvent contract."""
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
            payload={
                "to": event_data.get("phase", ""),
                "reason": event_data.get("reason", ""),
            },
        )
    if event_type == "diagnosis_result":
        return factory.next(
            channel="state",
            event_type="state.diagnosis.ready",
            payload={"diagnoses": event_data.get("diagnoses", [])},
        )
    if event_type == "treatment_result":
        return factory.next(
            channel="state",
            event_type="state.treatment.ready",
            payload={"treatment_plan": event_data.get("treatment_plan", {})},
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
    if event_type == "state.interaction.required":
        return factory.next(
            channel="state",
            event_type="state.interaction.required",
            payload={
                "interaction_id": event_data.get("interaction_id", ""),
                "question": event_data.get("question", {}),
            },
            ids=StreamEventIds(
                interaction_id=event_data.get("interaction_id") or None,
            ),
        )
    if event_type == "usage":
        return factory.next(
            channel="usage",
            event_type="usage.reported",
            payload={"usage": event_data.get("usage", {})},
        )

    return factory.next(
        channel="stream",
        event_type="stream.error",
        payload={"message": f"Unknown graph event type: {event_type}"},
    )
