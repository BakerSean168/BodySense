"""Default ToolRegistry for the consultation workflow."""

from __future__ import annotations

from .tool_executor import ToolExecutor
from .tool_registry import ToolRegistry
from .tools.ask_user import make_ask_user_tool
from .tools.extract_symptom_info import make_extract_symptom_info_tool
from .tools.get_posture_analysis import make_get_posture_analysis_tool
from .tools.record_answer_attribution import make_record_answer_attribution_tool
from .tools.record_lifestyle_context import make_record_lifestyle_context_tool
from .tools.search_knowledge import make_search_knowledge_tool

_default_registry: ToolRegistry | None = None
_default_executor: ToolExecutor | None = None


def get_consultation_registry() -> ToolRegistry:
    """Get or create the default consultation ToolRegistry."""
    global _default_registry
    if _default_registry is None:
        _default_registry = ToolRegistry()
        _default_registry.register(make_search_knowledge_tool())
        _default_registry.register(make_extract_symptom_info_tool())
        _default_registry.register(make_record_lifestyle_context_tool())
        _default_registry.register(make_get_posture_analysis_tool())
        _default_registry.register(make_record_answer_attribution_tool())
        _default_registry.register(make_ask_user_tool())
    return _default_registry


def get_consultation_executor() -> ToolExecutor:
    """Get or create the default consultation ToolExecutor."""
    global _default_executor
    if _default_executor is None:
        _default_executor = ToolExecutor(get_consultation_registry())
    return _default_executor
