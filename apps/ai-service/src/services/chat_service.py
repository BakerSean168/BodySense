"""Chat service for consultation conversations."""

import uuid
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any

from ..models.consultation import ChatContext


@dataclass
class NDJSONEvent:
    """A single NDJSON event to send to the client."""

    event_type: str  # "message" or "done"
    data: dict[str, Any]


@dataclass
class _StreamAccumulator:
    """Accumulates usage and text across the stream for the done event."""

    total_input_tokens: int = 0
    total_output_tokens: int = 0
    full_text: str = ""


class ChatService:
    """Service for managing consultation chat conversations."""

    async def stream_chat(
        self,
        context: ChatContext,
        user_message: str,
        rag_results: list[dict[str, Any]] | None = None,
    ) -> AsyncIterator[NDJSONEvent]:
        """
        Process a user message and stream the AI response.

        Delegates to the LangGraph consultation graph for explicit workflow
        orchestration. Each graph node emits events via StreamWriter, which
        are yielded as NDJSONEvent objects.

        Args:
            context: The chat session context.
            user_message: The user's message text.
            rag_results: Optional RAG search results for context.

        Yields:
            NDJSONEvent objects to send to the client.
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

        async for event_data in stream_consultation(graph_state):
            if event_data.get("type") == "__done__":
                # Build spec-compliant done event
                done_data = {
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
                yield NDJSONEvent(event_type="done", data=done_data)
            else:
                # Track text for usage accounting
                if event_data.get("type") == "text_delta":
                    acc.full_text += event_data.get("delta", "")
                elif event_data.get("type") == "usage":
                    usage = event_data.get("usage", {})
                    acc.total_input_tokens += usage.get("input_tokens", 0)
                    acc.total_output_tokens += usage.get("output_tokens", 0)
                yield NDJSONEvent(event_type="message", data=event_data)
