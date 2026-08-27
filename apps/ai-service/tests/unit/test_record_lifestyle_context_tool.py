import pytest

from src.services.agent.tool_types import ToolCategory
from src.services.agent.tools.record_lifestyle_context import (
    handle_record_lifestyle_context,
    make_record_lifestyle_context_tool,
)


@pytest.mark.asyncio
async def test_record_lifestyle_context_normalizes_explicit_user_state() -> None:
    result = await handle_record_lifestyle_context(
        {
            "section": "sleep",
            "summary": "最近换夜班，通常凌晨五点睡",
            "details": {"shift_work": True, "typical_sleep_start": "05:00"},
        }
    )
    assert result["section"] == "sleep"
    assert result["summary"] == "最近换夜班，通常凌晨五点睡"
    assert result["details"]["shift_work"] is True


@pytest.mark.asyncio
async def test_record_lifestyle_context_rejects_unknown_taxonomy() -> None:
    result = await handle_record_lifestyle_context(
        {"section": "occupation", "summary": "程序员"}
    )
    assert result == {"error": "unsupported lifestyle section"}


def test_lifestyle_tool_is_structured_query_event_not_direct_database_write() -> None:
    tool = make_record_lifestyle_context_tool()
    assert tool.name == "record_lifestyle_context"
    assert tool.category == ToolCategory.QUERY
    assert tool.required_params == ["section", "summary"]
