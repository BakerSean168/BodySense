"""ask_user tool — interrupt the run to request user input.

This tool is registered but NOT included in the default consultation
tool list passed to the LLM. It exists for future HITL flows where
the runtime explicitly opts in.
"""

from __future__ import annotations

from typing import Any

from ..tool_types import RuntimeToolDefinition, ToolCategory, ToolResult, ToolStatus

ASK_USER_SCHEMA: dict[str, Any] = {
    "name": "ask_user",
    "description": (
        "暂停当前流程，向用户提出一个问题并等待回答。"
        "仅在需要用户提供关键信息才能继续时使用。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "question": {
                "type": "string",
                "description": "向用户提出的问题",
            },
            "reason": {
                "type": "string",
                "description": "为什么需要向用户提问的原因",
            },
            "answer_type": {
                "type": "string",
                "enum": ["text", "single_choice", "multi_choice", "number", "date"],
                "description": "期望的回答类型",
                "default": "text",
            },
            "options": {
                "type": "array",
                "items": {"type": "string"},
                "description": "当 answer_type 为 single_choice 或 multi_choice 时的选项列表",
            },
            "required": {
                "type": "boolean",
                "description": "用户是否必须回答此问题",
                "default": True,
            },
            "context": {
                "type": "string",
                "description": "提供给用户的额外上下文信息",
            },
        },
        "required": ["question"],
    },
}


async def handle_ask_user(arguments: dict[str, Any]) -> ToolResult:
    """Handle an ask_user tool call.

    Always returns interrupted status — the orchestration layer must
    persist the interaction and pause the run. This handler never blocks
    waiting for a user response.
    """
    question = arguments.get("question", "").strip()
    if not question:
        return ToolResult(
            tool_call_id="",
            tool_name="ask_user",
            status=ToolStatus.FAILED,
            error="question is required",
        )

    answer_type = arguments.get("answer_type", "text")
    valid_types = {"text", "single_choice", "multi_choice", "number", "date"}
    if answer_type not in valid_types:
        return ToolResult(
            tool_call_id="",
            tool_name="ask_user",
            status=ToolStatus.FAILED,
            error=f"invalid answer_type: {answer_type}",
        )

    # Return interrupted — the run should pause here
    return ToolResult(
        tool_call_id="",
        tool_name="ask_user",
        status=ToolStatus.INTERRUPTED,
        content={
            "question": question,
            "reason": arguments.get("reason", ""),
            "answer_type": answer_type,
            "options": arguments.get("options", []),
            "required": arguments.get("required", True),
            "context": arguments.get("context", ""),
        },
    )


def make_ask_user_tool() -> RuntimeToolDefinition:
    """Create a RuntimeToolDefinition for ask_user."""
    return RuntimeToolDefinition(
        name=ASK_USER_SCHEMA["name"],
        description=ASK_USER_SCHEMA["description"],
        parameters=ASK_USER_SCHEMA["parameters"],
        category=ToolCategory.HUMAN,
        handler=handle_ask_user,
        required_params=["question"],
    )
