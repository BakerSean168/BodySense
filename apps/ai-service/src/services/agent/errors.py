"""Error types for the agent tool system."""


class ToolError(Exception):
    """Base error for tool operations."""

    def __init__(self, message: str, tool_name: str = ""):
        super().__init__(message)
        self.tool_name = tool_name


class ToolNotFoundError(ToolError):
    """Raised when a requested tool is not registered."""

    def __init__(self, tool_name: str):
        super().__init__(f"Tool not found: {tool_name}", tool_name=tool_name)


class ToolValidationError(ToolError):
    """Raised when tool arguments fail validation."""

    def __init__(self, tool_name: str, message: str):
        super().__init__(
            f"Validation failed for tool '{tool_name}': {message}",
            tool_name=tool_name,
        )


class ToolDuplicateError(ToolError):
    """Raised when registering a tool with a name that already exists."""

    def __init__(self, tool_name: str):
        super().__init__(
            f"Tool already registered: {tool_name}",
            tool_name=tool_name,
        )
