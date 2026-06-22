"""Chat service for consultation conversations."""

from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any

from ..models.consultation import ChatContext
from ..prompts.consultation import (
    SYMPTOM_EXTRACTION_TOOL,
    build_rag_context,
    format_profile_context,
    get_system_prompt,
)
from .llm_provider import ChatMessage, ToolCall, get_llm_provider

# Maximum conversation turns to keep in context
MAX_CONTEXT_TURNS = 10


@dataclass
class SSEEvent:
    """An SSE event to send to the client."""

    event_type: str  # "message" or "done"
    data: dict[str, Any]


class ChatService:
    """Service for managing consultation chat conversations."""

    async def stream_chat(
        self,
        context: ChatContext,
        user_message: str,
        rag_results: list[dict[str, Any]] | None = None,
    ) -> AsyncIterator[SSEEvent]:
        """
        Process a user message and stream the AI response.

        Args:
            context: The chat session context.
            user_message: The user's message text.
            rag_results: Optional RAG search results for context.

        Yields:
            SSEEvent objects to send to the client.
        """
        provider = get_llm_provider()

        # Build messages list
        messages = self._build_messages(context, user_message, rag_results)

        # Define tools for function calling
        tools = [
            {
                "name": "extract_symptom_info",
                "description": SYMPTOM_EXTRACTION_TOOL["description"],
                "parameters": SYMPTOM_EXTRACTION_TOOL["parameters"],
            }
        ]

        # Stream the response
        accumulated_text = ""
        accumulated_tool_calls: list[ToolCall] = []

        async for chunk in provider.chat_stream(
            messages=messages,
            tools=[self._to_tool_def(t) for t in tools],
            temperature=0.7,
            max_tokens=2048,
        ):
            if chunk.delta:
                accumulated_text += chunk.delta
                yield SSEEvent(
                    event_type="message",
                    data={"type": "text", "content": chunk.delta},
                )

            if chunk.tool_calls:
                accumulated_tool_calls.extend(chunk.tool_calls)

            if chunk.finished:
                # Process any tool calls
                for tc in accumulated_tool_calls:
                    if tc.name == "extract_symptom_info":
                        # Update extracted info
                        context.extracted_info.add_symptom(tc.arguments)
                        yield SSEEvent(
                            event_type="message",
                            data={
                                "type": "extracted_info",
                                "info": tc.arguments,
                            },
                        )

        # Send done event
        yield SSEEvent(
            event_type="done",
            data={
                "session_id": context.session_id,
                "full_text": accumulated_text,
                "extracted_info": context.extracted_info.to_dict(),
            },
        )

    def _build_messages(
        self,
        context: ChatContext,
        user_message: str,
        rag_results: list[dict[str, Any]] | None = None,
    ) -> list[ChatMessage]:
        """Build the messages list for the LLM."""
        messages: list[ChatMessage] = []

        # System prompt with profile context
        profile_context = format_profile_context(context.profile)
        system_content = get_system_prompt(profile_context)

        # Add RAG context if available
        if rag_results:
            rag_context = build_rag_context(rag_results)
            system_content += f"\n\n{rag_context}"

        # Add extracted info summary to system prompt
        if context.extracted_info.symptoms:
            info_lines = ["## 已提取的症状信息"]
            for s in context.extracted_info.symptoms:
                line = f"- {s.body_part}：{s.symptom_type or '待补充'}"
                if s.duration:
                    line += f"，持续{s.duration}"
                if s.trigger:
                    line += f"，{s.trigger}时出现"
                info_lines.append(line)
            system_content += "\n\n" + "\n".join(info_lines)

        messages.append(ChatMessage(role="system", content=system_content))

        # Add conversation history (keep last N turns)
        history = context.messages[-MAX_CONTEXT_TURNS * 2:]  # user + assistant pairs
        for msg in history:
            role = msg.get("role", "user")
            content = msg.get("content", "")
            messages.append(ChatMessage(role=role, content=content))

        # Add current user message
        messages.append(ChatMessage(role="user", content=user_message))

        return messages

    def _to_tool_def(self, tool_dict: dict[str, Any]):
        """Convert tool dict to ToolDefinition."""
        from .llm_provider import ToolDefinition

        return ToolDefinition(
            name=tool_dict["name"],
            description=tool_dict["description"],
            parameters=tool_dict["parameters"],
        )
