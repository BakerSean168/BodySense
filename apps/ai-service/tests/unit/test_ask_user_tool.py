"""Tests for the ask_user tool."""

import pytest

from src.services.agent.tool_types import ToolCategory, ToolStatus
from src.services.agent.tools.ask_user import handle_ask_user, make_ask_user_tool


@pytest.mark.asyncio
async def test_handle_ask_user_returns_interrupted():
    result = await handle_ask_user({"question": "你的年龄是？"})
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["question"] == "你的年龄是？"
    assert result.content["answer_type"] == "text"


@pytest.mark.asyncio
async def test_handle_ask_user_with_options():
    result = await handle_ask_user({
        "question": "选择你的症状类型",
        "answer_type": "single_choice",
        "options": ["疼痛", "酸胀", "麻木"],
    })
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["answer_type"] == "single_choice"
    assert result.content["options"] == ["疼痛", "酸胀", "麻木"]


@pytest.mark.asyncio
async def test_handle_ask_user_empty_question_fails():
    result = await handle_ask_user({"question": ""})
    assert result.status == ToolStatus.FAILED
    assert "question is required" in result.error


@pytest.mark.asyncio
async def test_handle_ask_user_invalid_answer_type():
    result = await handle_ask_user({
        "question": "test",
        "answer_type": "invalid_type",
    })
    assert result.status == ToolStatus.FAILED
    assert "invalid answer_type" in result.error


@pytest.mark.asyncio
async def test_handle_ask_user_never_blocks():
    """ask_user must return immediately without waiting for user input."""
    import asyncio

    # Should complete within a short timeout
    result = await asyncio.wait_for(
        handle_ask_user({"question": "test"}),
        timeout=1.0,
    )
    assert result.status == ToolStatus.INTERRUPTED


def test_make_ask_user_tool():
    tool = make_ask_user_tool()
    assert tool.name == "ask_user"
    assert tool.category == ToolCategory.HUMAN
    assert tool.required_params == ["question"]
    assert tool.handler is not None
