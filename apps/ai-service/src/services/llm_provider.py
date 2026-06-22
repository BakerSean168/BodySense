"""LLM Provider abstraction for multi-model support."""

import json
import os
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any, Protocol


@dataclass
class ToolDefinition:
    """Definition of a tool for function calling."""

    name: str
    description: str
    parameters: dict[str, Any]


@dataclass
class ToolCall:
    """A tool call from the LLM."""

    id: str
    name: str
    arguments: dict[str, Any]


@dataclass
class ChatMessage:
    """A chat message."""

    role: str  # "system", "user", "assistant", "tool"
    content: str | None = None
    tool_calls: list[ToolCall] | None = None
    tool_call_id: str | None = None


@dataclass
class StreamChunk:
    """A chunk from streaming response."""

    delta: str = ""
    tool_calls: list[ToolCall] | None = None
    finished: bool = False


class LLMProvider(Protocol):
    """Protocol for LLM providers."""

    async def chat(
        self,
        messages: list[ChatMessage],
        tools: list[ToolDefinition] | None = None,
        temperature: float = 0.7,
        max_tokens: int = 2048,
    ) -> ChatMessage:
        """Non-streaming chat completion."""
        ...

    async def chat_stream(
        self,
        messages: list[ChatMessage],
        tools: list[ToolDefinition] | None = None,
        temperature: float = 0.7,
        max_tokens: int = 2048,
    ) -> AsyncIterator[StreamChunk]:
        """Streaming chat completion."""
        ...


class OpenAICompatibleProvider:
    """LLM provider using OpenAI-compatible API (works with Qwen, DeepSeek, etc.)."""

    def __init__(
        self,
        model: str | None = None,
        api_key: str | None = None,
        base_url: str | None = None,
    ):
        from openai import AsyncOpenAI

        self.model = model or os.getenv("LLM_MODEL", "gpt-4o-mini")
        self.api_key = (
            api_key or os.getenv("LLM_API_KEY") or os.getenv("OPENAI_API_KEY")
        )
        self.base_url = base_url or os.getenv("LLM_BASE_URL")

        if not self.api_key:
            raise ValueError("LLM_API_KEY or OPENAI_API_KEY is required")

        kwargs: dict[str, Any] = {"api_key": self.api_key}
        if self.base_url:
            kwargs["base_url"] = self.base_url
        self._client = AsyncOpenAI(**kwargs)

    def _convert_messages(self, messages: list[ChatMessage]) -> list[dict[str, Any]]:
        """Convert ChatMessage to OpenAI API format."""
        result = []
        for msg in messages:
            m: dict[str, Any] = {"role": msg.role}
            if msg.content is not None:
                m["content"] = msg.content
            if msg.tool_calls:
                m["tool_calls"] = [
                    {
                        "id": tc.id,
                        "type": "function",
                        "function": {
                            "name": tc.name,
                            "arguments": json.dumps(tc.arguments, ensure_ascii=False),
                        },
                    }
                    for tc in msg.tool_calls
                ]
            if msg.tool_call_id:
                m["tool_call_id"] = msg.tool_call_id
            result.append(m)
        return result

    def _convert_tools(self, tools: list[ToolDefinition]) -> list[dict[str, Any]]:
        """Convert ToolDefinition to OpenAI API format."""
        return [
            {
                "type": "function",
                "function": {
                    "name": t.name,
                    "description": t.description,
                    "parameters": t.parameters,
                },
            }
            for t in tools
        ]

    def _parse_tool_calls(self, tool_calls: Any) -> list[ToolCall]:
        """Parse OpenAI tool calls to ToolCall."""
        if not tool_calls:
            return []
        result = []
        for tc in tool_calls:
            args = tc.function.arguments
            if isinstance(args, str):
                args = json.loads(args)
            result.append(
                ToolCall(id=tc.id, name=tc.function.name, arguments=args)
            )
        return result

    async def chat(
        self,
        messages: list[ChatMessage],
        tools: list[ToolDefinition] | None = None,
        temperature: float = 0.7,
        max_tokens: int = 2048,
    ) -> ChatMessage:
        """Non-streaming chat completion."""
        kwargs: dict[str, Any] = {
            "model": self.model,
            "messages": self._convert_messages(messages),
            "temperature": temperature,
            "max_tokens": max_tokens,
        }
        if tools:
            kwargs["tools"] = self._convert_tools(tools)

        response = await self._client.chat.completions.create(**kwargs)
        choice = response.choices[0]
        msg = choice.message

        return ChatMessage(
            role="assistant",
            content=msg.content,
            tool_calls=self._parse_tool_calls(msg.tool_calls),
        )

    async def chat_stream(
        self,
        messages: list[ChatMessage],
        tools: list[ToolDefinition] | None = None,
        temperature: float = 0.7,
        max_tokens: int = 2048,
    ) -> AsyncIterator[StreamChunk]:
        """Streaming chat completion."""
        kwargs: dict[str, Any] = {
            "model": self.model,
            "messages": self._convert_messages(messages),
            "temperature": temperature,
            "max_tokens": max_tokens,
            "stream": True,
        }
        if tools:
            kwargs["tools"] = self._convert_tools(tools)

        stream = await self._client.chat.completions.create(**kwargs)

        # Accumulate tool calls across chunks
        accumulated_tool_calls: dict[int, dict[str, Any]] = {}

        async for chunk in stream:
            if not chunk.choices:
                continue

            delta = chunk.choices[0].delta
            finish_reason = chunk.choices[0].finish_reason

            # Handle text content
            text_delta = delta.content or ""

            # Handle tool calls
            tool_calls = None
            if delta.tool_calls:
                for tc_delta in delta.tool_calls:
                    idx = tc_delta.index
                    if idx not in accumulated_tool_calls:
                        accumulated_tool_calls[idx] = {
                            "id": "",
                            "name": "",
                            "arguments": "",
                        }
                    acc = accumulated_tool_calls[idx]
                    if tc_delta.id:
                        acc["id"] = tc_delta.id
                    if tc_delta.function:
                        if tc_delta.function.name:
                            acc["name"] = tc_delta.function.name
                        if tc_delta.function.arguments:
                            acc["arguments"] += tc_delta.function.arguments

            is_finished = finish_reason == "stop" or finish_reason == "tool_calls"

            # On finish, parse accumulated tool calls
            if is_finished and accumulated_tool_calls:
                tool_calls = []
                for idx in sorted(accumulated_tool_calls.keys()):
                    acc = accumulated_tool_calls[idx]
                    args = acc["arguments"]
                    if isinstance(args, str):
                        args = json.loads(args) if args else {}
                    tool_calls.append(
                        ToolCall(id=acc["id"], name=acc["name"], arguments=args)
                    )

            yield StreamChunk(
                delta=text_delta,
                tool_calls=tool_calls,
                finished=is_finished,
            )


# Singleton instance
_default_provider: OpenAICompatibleProvider | None = None


def get_llm_provider() -> OpenAICompatibleProvider:
    """Get or create the default LLM provider."""
    global _default_provider
    if _default_provider is None:
        _default_provider = OpenAICompatibleProvider()
    return _default_provider
