"""ToolRegistry — registration and lookup for agent tools."""

from __future__ import annotations

from ...ai.types import ToolDefinition
from .errors import ToolDuplicateError, ToolNotFoundError
from .tool_types import RuntimeToolDefinition


class ToolRegistry:
    """Registry for runtime tool definitions.

    Tools are registered by name. Duplicate names are rejected.
    Provider-level ToolDefinitions can be generated for LLM function calling.
    """

    def __init__(self) -> None:
        self._tools: dict[str, RuntimeToolDefinition] = {}

    def register(self, tool: RuntimeToolDefinition) -> None:
        """Register a tool. Raises ToolDuplicateError if name already exists."""
        if tool.name in self._tools:
            raise ToolDuplicateError(tool.name)
        self._tools[tool.name] = tool

    def get(self, name: str) -> RuntimeToolDefinition:
        """Get a tool by name. Raises ToolNotFoundError if not found."""
        tool = self._tools.get(name)
        if tool is None:
            raise ToolNotFoundError(name)
        return tool

    def list_tools(self) -> list[RuntimeToolDefinition]:
        """Return all registered tools."""
        return list(self._tools.values())

    def to_provider_tools(self) -> list[ToolDefinition]:
        """Convert all registered tools to provider-level ToolDefinitions."""
        return [t.to_provider_tool() for t in self._tools.values()]

    def has(self, name: str) -> bool:
        """Check if a tool is registered."""
        return name in self._tools
