"""Runtime-level tool types for the agent tool system."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Any

from ...ai.types import ToolDefinition


class ToolCategory(str, Enum):
    """Category of a tool — determines execution policy."""

    QUERY = "query"  # Pure read, no side effects
    WRITE = "write"  # Mutates state
    HUMAN = "human"  # Requires human interaction
    DANGEROUS = "dangerous"  # Needs confirmation


class ToolStatus(str, Enum):
    """Outcome of a tool execution."""

    SUCCESS = "success"
    FAILED = "failed"
    INTERRUPTED = "interrupted"
    REQUIRES_CONFIRMATION = "requires_confirmation"


@dataclass
class RuntimeToolDefinition:
    """Extended tool definition with runtime metadata.

    Wraps the provider-level ToolDefinition with category, handler,
    and validation metadata.
    """

    name: str
    description: str
    parameters: dict[str, Any]
    category: ToolCategory = ToolCategory.QUERY
    handler: Any = None  # async (args: dict) -> dict | ToolResult
    required_params: list[str] = field(default_factory=list)

    def to_provider_tool(self) -> ToolDefinition:
        """Convert to the provider-level ToolDefinition."""
        return ToolDefinition(
            name=self.name,
            description=self.description,
            parameters=self.parameters,
        )


@dataclass
class ToolResult:
    """Structured result from tool execution."""

    tool_call_id: str
    tool_name: str
    status: ToolStatus
    content: dict[str, Any] | str | None = None
    error: str | None = None
    interaction_id: str | None = None

    def to_dict(self) -> dict[str, Any]:
        """Serialize to dict for SSE events and LLM tool messages."""
        d: dict[str, Any] = {
            "tool_call_id": self.tool_call_id,
            "tool_name": self.tool_name,
            "status": self.status.value,
        }
        if self.content is not None:
            d["content"] = self.content
        if self.error is not None:
            d["error"] = self.error
        if self.interaction_id is not None:
            d["interaction_id"] = self.interaction_id
        return d
