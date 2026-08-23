"""Tests for ToolRegistry."""

import pytest

from src.services.agent.errors import ToolDuplicateError, ToolNotFoundError
from src.services.agent.tool_registry import ToolRegistry
from src.services.agent.tool_types import RuntimeToolDefinition, ToolCategory


def _make_tool(name: str = "test_tool") -> RuntimeToolDefinition:
    return RuntimeToolDefinition(
        name=name,
        description="A test tool",
        parameters={"type": "object", "properties": {"q": {"type": "string"}}},
        category=ToolCategory.QUERY,
        handler=None,
    )


def test_register_and_get():
    registry = ToolRegistry()
    tool = _make_tool()
    registry.register(tool)
    assert registry.get("test_tool") is tool


def test_get_unknown_tool_raises():
    registry = ToolRegistry()
    with pytest.raises(ToolNotFoundError):
        registry.get("nonexistent")


def test_duplicate_registration_raises():
    registry = ToolRegistry()
    registry.register(_make_tool())
    with pytest.raises(ToolDuplicateError):
        registry.register(_make_tool())


def test_list_tools():
    registry = ToolRegistry()
    registry.register(_make_tool("a"))
    registry.register(_make_tool("b"))
    tools = registry.list_tools()
    assert len(tools) == 2
    names = {t.name for t in tools}
    assert names == {"a", "b"}


def test_to_provider_tools():
    registry = ToolRegistry()
    registry.register(_make_tool("search"))
    provider_tools = registry.to_provider_tools()
    assert len(provider_tools) == 1
    assert provider_tools[0].name == "search"
    assert provider_tools[0].description == "A test tool"


def test_has():
    registry = ToolRegistry()
    assert not registry.has("test_tool")
    registry.register(_make_tool())
    assert registry.has("test_tool")


def test_default_consultation_registry_includes_answer_attribution_tool():
    from src.services.agent.consultation_tools import get_consultation_registry

    registry = get_consultation_registry()
    assert registry.has("record_answer_attribution")
    tool = registry.get("record_answer_attribution")
    assert tool.category == ToolCategory.QUERY
    assert tool.required_params == ["claims"]
