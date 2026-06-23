"""Chat service for consultation conversations."""

import re
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
        try:
            provider = get_llm_provider()
        except ValueError:
            fallback_text = self._build_fallback_reply(user_message, rag_results)
            accumulated_text = ""
            for chunk in self._chunk_text(fallback_text):
                accumulated_text += chunk
                yield SSEEvent(
                    event_type="message",
                    data={"type": "text", "content": chunk},
                )

            yield SSEEvent(
                event_type="done",
                data={
                    "session_id": context.session_id,
                    "full_text": accumulated_text,
                    "extracted_info": context.extracted_info.to_dict(),
                },
            )
            return

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

    def _build_fallback_reply(
        self,
        user_message: str,
        rag_results: list[dict[str, Any]] | None = None,
    ) -> str:
        """Build a deterministic reply when no online LLM is configured."""
        if not rag_results:
            return (
                "我已经收到你的描述，但当前本地环境没有配置云端大模型，且知识库里暂时没有检索到足够匹配的条目。\n"
                "你可以继续补充具体部位、动作场景、是否双侧对称，以及持续多久，我会继续按本地知识库帮你缩小范围。"
            )

        top_result = rag_results[0]
        title = top_result.get("title", "相关体态问题")
        summary = top_result.get("summary", "").strip()
        content = (
            top_result.get("body_markdown")
            or top_result.get("content")
            or summary
            or ""
        )
        plain_content = self._markdown_to_text(content)
        lines = [f"根据当前本地知识库，你提到的问题最接近“{title}”。"]

        if summary:
            lines.append(f"核心判断：{summary}")

        if plain_content:
            lines.append(f"知识要点：{plain_content[:280]}")

        clips = top_result.get("clips") or []
        if clips:
            clip_titles = [
                clip.get("title", "").strip()
                for clip in clips[:2]
                if clip.get("title")
            ]
            if clip_titles:
                lines.append(f"可参考的动作演示：{'、'.join(clip_titles)}。")

        if len(rag_results) > 1:
            extra_titles = [
                result.get("title", "").strip()
                for result in rag_results[1:3]
                if result.get("title")
            ]
            if extra_titles:
                lines.append(f"我同时参考了：{'、'.join(extra_titles)}。")

        lines.append("当前回答来自本地 curated 知识库整理，不构成医疗诊断；")
        lines.append("如果你愿意，我可以继续根据你的具体症状帮你细化判断。")
        return "\n".join(lines)

    def _markdown_to_text(self, content: str) -> str:
        """Flatten markdown into readable plain text for chat bubbles."""
        text = re.sub(r"^#+\s*", "", content, flags=re.MULTILINE)
        text = re.sub(r"^[*-]\s*", "", text, flags=re.MULTILINE)
        text = re.sub(r"\n{2,}", "\n", text)
        return text.strip()

    def _chunk_text(self, text: str, chunk_size: int = 120) -> list[str]:
        """Split a reply into stream-friendly chunks."""
        return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size)]
