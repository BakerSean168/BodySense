"""ToolExecutor — validates arguments and executes registered tools."""

from __future__ import annotations

import logging
from typing import Any

from .errors import ToolNotFoundError
from .tool_registry import ToolRegistry
from .tool_types import RuntimeToolDefinition, ToolResult, ToolStatus

logger = logging.getLogger(__name__)


class ToolExecutor:
    """Executes tools with argument validation and structured error handling."""

    def __init__(self, registry: ToolRegistry) -> None:
        self._registry = registry

    async def execute(
        self,
        tool_call_id: str,
        tool_name: str,
        arguments: dict[str, Any],
    ) -> ToolResult:
        """Execute a tool by name with argument validation.

        Returns a ToolResult with status=success or status=failed.
        Never raises — exceptions are wrapped into ToolResult.
        """
        # Lookup
        try:
            tool = self._registry.get(tool_name)
        except ToolNotFoundError:
            return ToolResult(
                tool_call_id=tool_call_id,
                tool_name=tool_name,
                status=ToolStatus.FAILED,
                error=f"Unknown tool: {tool_name}",
            )

        # Validate required params
        validation_error = self._validate(tool, arguments)
        if validation_error:
            return ToolResult(
                tool_call_id=tool_call_id,
                tool_name=tool_name,
                status=ToolStatus.FAILED,
                error=validation_error,
            )

        # Execute handler
        try:
            result = await tool.handler(arguments)
            # If handler already returns a ToolResult, use it
            if isinstance(result, ToolResult):
                return result
            # Otherwise wrap the result
            return ToolResult(
                tool_call_id=tool_call_id,
                tool_name=tool_name,
                status=ToolStatus.SUCCESS,
                content=result if isinstance(result, dict) else {"result": result},
            )
        except Exception as e:
            logger.exception("Tool handler failed: %s", tool_name)
            return ToolResult(
                tool_call_id=tool_call_id,
                tool_name=tool_name,
                status=ToolStatus.FAILED,
                error=str(e),
            )

    def _validate(self, tool: RuntimeToolDefinition, arguments: dict[str, Any]) -> str | None:
        """Validate arguments against tool definition. Returns error message or None."""
        # Check required params
        for param in tool.required_params:
            if param not in arguments or arguments[param] is None:
                return f"Missing required parameter: {param}"
            # Check string params are not empty
            if isinstance(arguments[param], str) and not arguments[param].strip():
                return f"Empty required parameter: {param}"

        # Check for type mismatches on known param types from the schema
        props = tool.parameters.get("properties", {})
        for key, value in arguments.items():
            if key not in props or value is None:
                continue
            expected_type = props[key].get("type")
            if expected_type == "string" and not isinstance(value, str):
                return f"Parameter '{key}' must be a string"
            if expected_type == "integer" and not isinstance(value, int):
                # Allow float that is actually an int
                if isinstance(value, float) and value.is_integer():
                    arguments[key] = int(value)
                else:
                    return f"Parameter '{key}' must be an integer"

        return None
