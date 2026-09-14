"""Core types for the AI provider system."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal, TypeAlias


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


@dataclass(frozen=True, slots=True)
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
    # North-Star: when set, these override the use_case route lookup so the
    # exact immutable manifest configuration is honored end-to-end.
    logical_model: str | None = None
    model_settings: dict[str, Any] | None = None


@dataclass
class AiResponse:
    text: str
    model: str
    provider: str
    usage: TokenUsage | None = None
    finish_reason: str | None = None
    tool_calls: list[ToolCall] | None = None
    raw: Any = None


@dataclass(frozen=True, slots=True)
class AiTextDeltaEvent:
    """A validated text fragment from the provider stream."""

    text: str
    type: Literal["text_delta"] = field(init=False, default="text_delta")


@dataclass(frozen=True, slots=True)
class AiToolCallDoneEvent:
    """A complete tool call with provider JSON already parsed as an object."""

    tool_call_id: str
    tool_name: str
    tool_arguments: dict[str, Any]
    type: Literal["tool_call_done"] = field(init=False, default="tool_call_done")


@dataclass(frozen=True, slots=True)
class AiUsageEvent:
    """Token usage reported by the provider stream."""

    usage: TokenUsage
    type: Literal["usage"] = field(init=False, default="usage")


@dataclass(frozen=True, slots=True)
class AiDoneEvent:
    """Provider stream completion marker."""

    finish_reason: str | None
    type: Literal["done"] = field(init=False, default="done")


AiStreamEvent: TypeAlias = AiTextDeltaEvent | AiToolCallDoneEvent | AiUsageEvent | AiDoneEvent
