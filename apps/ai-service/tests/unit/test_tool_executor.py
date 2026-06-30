"""Tests for ToolExecutor."""

import pytest

from src.services.agent.tool_executor import ToolExecutor
from src.services.agent.tool_registry import ToolRegistry
from src.services.agent.tool_types import (
    RuntimeToolDefinition,
    ToolCategory,
    ToolResult,
    ToolStatus,
)


def _make_tool(name="test_tool", handler=None, required=None):
    if handler is None:

        async def _ok_handler(args):
            return {"echo": args.get("q", "")}

        handler = _ok_handler
    return RuntimeToolDefinition(
        name=name,
        description="test",
        parameters={
            "type": "object",
            "properties": {
                "q": {"type": "string"},
                "n": {"type": "integer"},
            },
        },
        category=ToolCategory.QUERY,
        handler=handler,
        required_params=required or [],
    )


@pytest.mark.asyncio
async def test_execute_unknown_tool():
    registry = ToolRegistry()
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "nonexistent", {})
    assert result.status == ToolStatus.FAILED
    assert "Unknown tool" in result.error


@pytest.mark.asyncio
async def test_execute_successful():
    registry = ToolRegistry()
    registry.register(_make_tool())
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "test_tool", {"q": "hello"})
    assert result.status == ToolStatus.SUCCESS
    assert result.content == {"echo": "hello"}


@pytest.mark.asyncio
async def test_execute_missing_required_param():
    registry = ToolRegistry()
    registry.register(_make_tool(required=["q"]))
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "test_tool", {})
    assert result.status == ToolStatus.FAILED
    assert "Missing required parameter" in result.error


@pytest.mark.asyncio
async def test_execute_empty_required_string():
    registry = ToolRegistry()
    registry.register(_make_tool(required=["q"]))
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "test_tool", {"q": "  "})
    assert result.status == ToolStatus.FAILED
    assert "Empty required parameter" in result.error


@pytest.mark.asyncio
async def test_execute_handler_exception():
    async def _bad_handler(args):
        raise ValueError("something broke")

    registry = ToolRegistry()
    registry.register(_make_tool(handler=_bad_handler))
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "test_tool", {})
    assert result.status == ToolStatus.FAILED
    assert "something broke" in result.error


@pytest.mark.asyncio
async def test_execute_handler_returns_tool_result():
    async def _custom_handler(args):
        return ToolResult(
            tool_call_id="tc1",
            tool_name="test_tool",
            status=ToolStatus.INTERRUPTED,
            content={"reason": "need input"},
        )

    registry = ToolRegistry()
    registry.register(_make_tool(handler=_custom_handler))
    executor = ToolExecutor(registry)
    result = await executor.execute("tc1", "test_tool", {})
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content == {"reason": "need input"}


@pytest.mark.asyncio
async def test_execute_type_validation_integer():
    async def _ok(args):
        return args

    registry = ToolRegistry()
    registry.register(_make_tool(handler=_ok))
    executor = ToolExecutor(registry)
    # Float that is an int should be coerced
    result = await executor.execute("tc1", "test_tool", {"n": 3.0})
    assert result.status == ToolStatus.SUCCESS
    assert result.content["n"] == 3

    # Non-integer float should fail
    result2 = await executor.execute("tc2", "test_tool", {"n": 3.5})
    assert result2.status == ToolStatus.FAILED
    assert "integer" in result2.error
