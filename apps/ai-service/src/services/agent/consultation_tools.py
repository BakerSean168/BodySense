"""Default ToolRegistry for the consultation workflow."""

from __future__ import annotations

import os

from .tool_executor import ToolExecutor
from .tool_registry import ToolRegistry
from .tools.ask_user import make_ask_user_tool
from .tools.extract_symptom_info import make_extract_symptom_info_tool
from .tools.search_knowledge import make_search_knowledge_tool

_default_registry: ToolRegistry | None = None
_default_executor: ToolExecutor | None = None


def get_consultation_registry() -> ToolRegistry:
    """Get or create the default consultation ToolRegistry.

    ask_user is gated behind ASK_USER_ENABLED env var (default: "false").
    Set ASK_USER_ENABLED=true to enable HITL interruption in consultation flows.
    """
    global _default_registry
    if _default_registry is None:
        _default_registry = ToolRegistry()
        _default_registry.register(make_search_knowledge_tool())
        _default_registry.register(make_extract_symptom_info_tool())

        if os.environ.get("ASK_USER_ENABLED", "false").lower() == "true":
            _default_registry.register(make_ask_user_tool())
    return _default_registry


def get_consultation_executor() -> ToolExecutor:
    """Get or create the default consultation ToolExecutor."""
    global _default_executor
    if _default_executor is None:
        _default_executor = ToolExecutor(get_consultation_registry())
    return _default_executor
