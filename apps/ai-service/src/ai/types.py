"""Core types for the AI provider system."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


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
    content: str | list[dict[str, Any]] | None = None
    tool_calls: list[ToolCall] | None = None
    tool_call_id: str | None = None


@dataclass
class TokenUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    total_tokens: int = 0


@dataclass
class AiRequest:
    use_case: str  # BodySense logical gateway route, never a physical provider/model
    messages: list[ChatMessage]
    tools: list[ToolDefinition] | None = None
    stream: bool = False
    response_format: str | None = None
    temperature: float | None = None
    max_tokens: int | None = None
    metadata: dict[str, Any] | None = None


@dataclass
class AiResponse:
    text: str
    model: str
    provider: str
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    tool_calls: list[ToolCall] | None = None
    raw: Any = None


@dataclass
class AiStreamEvent:
    type: str  # "text_delta" | "tool_call_done" | "usage" | "done" | "error"
    text: str | None = None
    tool_call_id: str | None = None
    tool_name: str | None = None
    tool_arguments: dict | None = None
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    error: str | None = None
