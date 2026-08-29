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
    assert result.content["context"]


@pytest.mark.asyncio
async def test_handle_ask_user_with_options():
    result = await handle_ask_user(
        {
            "question": "选择你的症状类型",
            "answer_type": "single_choice",
            "options": ["疼痛", "酸胀", "麻木"],
        }
    )
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["answer_type"] == "single_choice"
    assert result.content["options"] == ["疼痛", "酸胀", "麻木"]
    assert result.content["allow_custom_input"] is True
    assert result.content["context"]


@pytest.mark.asyncio
async def test_handle_ask_user_converts_yes_no_question_to_single_choice():
    result = await handle_ask_user({"question": "你是否经常感到颈部僵硬或疼痛？"})
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["question"] == "你是否经常感到颈部僵硬或疼痛？"
    assert result.content["answer_type"] == "single_choice"
    assert result.content["options"] == ["是", "否"]
    assert "帮助" in result.content["context"] or "确认" in result.content["context"]


@pytest.mark.asyncio
async def test_handle_ask_user_preserves_explicit_context():
    result = await handle_ask_user(
        {
            "question": "你是否感觉到颈部或肩部不适？",
            "context": "这能帮助我区分姿态观察和已经伴随不适的情况。",
        }
    )
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["context"] == "这能帮助我区分姿态观察和已经伴随不适的情况。"


@pytest.mark.asyncio
async def test_handle_ask_user_keeps_only_first_numbered_question():
    result = await handle_ask_user(
        {
            "question": (
                "为了更准确地分析你的头前移情况，请告诉我以下细节："
                "1. 你是否经常感到颈部或肩部僵硬或疼痛？ "
                "2. 是否有伴随头痛？ "
                "3. 是否长时间使用电脑？"
            )
        }
    )
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["question"] == "你是否经常感到颈部或肩部僵硬或疼痛？"
    assert result.content["answer_type"] == "single_choice"
    assert result.content["options"] == ["是", "否"]


@pytest.mark.asyncio
async def test_handle_ask_user_empty_question_fails():
    result = await handle_ask_user({"question": ""})
    assert result.status == ToolStatus.FAILED
    assert "question is required" in result.error


@pytest.mark.asyncio
async def test_handle_ask_user_invalid_answer_type():
    result = await handle_ask_user(
        {
            "question": "test",
            "answer_type": "invalid_type",
        }
    )
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


@pytest.mark.asyncio
async def test_handle_ask_user_preserves_runtime_owned_symptom_binding():
    capture_id = "0123456789abcdef01234567"
    result = await handle_ask_user(
        {
            "question": "请补充症状信息",
            "purpose": "symptom_intake",
            "fields": [
                {
                    "key": "duration",
                    "label": "持续多久",
                    "answer_type": "single_choice",
                    "options": ["2–7天", "1–4周"],
                }
            ],
            "state_binding": {
                "capture_id": capture_id,
                "seed_info": {
                    "capture_id": capture_id,
                    "body_part": "右臀",
                    "symptom_type": "疼痛",
                },
                "field_map": {"duration": "duration"},
            },
        }
    )
    assert result.status == ToolStatus.INTERRUPTED
    assert result.content["purpose"] == "symptom_intake"
    assert result.content["state_binding"]["revision"] == "symptom-intake-binding-v1"
    assert result.content["state_binding"]["capture_id"] == capture_id


@pytest.mark.asyncio
async def test_handle_ask_user_multi_field_form():
    result = await handle_ask_user(
        {
            "question": "请补充以下信息",
            "fields": [
                {"key": "body_part", "label": "不适部位", "answer_type": "text"},
                {
                    "key": "symmetric",
                    "label": "是否双侧对称",
                    "answer_type": "single_choice",
                    "options": ["是", "否"],
                },
                {"key": "duration", "label": "持续多久", "answer_type": "text"},
                {"key": "extra", "label": "应被截断", "answer_type": "text"},
            ],
        }
    )
    assert result.status == ToolStatus.INTERRUPTED
    fields = result.content["fields"]
    assert len(fields) == 3
    assert fields[0]["key"] == "body_part"
    assert fields[1]["options"] == ["是", "否"]
