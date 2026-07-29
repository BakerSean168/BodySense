"""ask_user tool — interrupt the run to request user input.

Registered in the default consultation tool list (see
`consultation_tools.py`) and exposed to the LLM. The active runtime
(`runtime/consultation_thread.py`) turns an INTERRUPTED result into a
LangGraph `interrupt()`, which pauses the thread until the user answers.
"""

from __future__ import annotations

import re
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
                "description": (
                    "当 answer_type 为 single_choice 或 multi_choice 时的选项列表，"
                    "优先提供 2-4 个简短选项"
                ),
            },
            "allow_custom_input": {
                "type": "boolean",
                "description": "是否允许用户在现成选项之外自行输入补充回答",
                "default": True,
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
            "fields": {
                "type": "array",
                "description": (
                    "结构化多字段表单（可选）。提供后前端一次收集多字段；"
                    "单问路径保持兼容（不传 fields）。最多 3 个字段。"
                ),
                "maxItems": 3,
                "items": {
                    "type": "object",
                    "properties": {
                        "key": {
                            "type": "string",
                            "description": "字段标识，用于答案结构化落库",
                        },
                        "label": {
                            "type": "string",
                            "description": "展示给用户的字段标签",
                        },
                        "answer_type": {
                            "type": "string",
                            "enum": [
                                "text",
                                "single_choice",
                                "multi_choice",
                                "number",
                                "date",
                                "scale",
                            ],
                            "default": "text",
                        },
                        "options": {
                            "type": "array",
                            "items": {"type": "string"},
                        },
                        "required": {"type": "boolean", "default": True},
                    },
                    "required": ["key", "label"],
                },
            },
        },
        "required": ["question"],
    },
}

YES_NO_QUESTION_RE = re.compile(r"(是否|有无|有没有|是不是|会不会|能否)")
NUMBERED_QUESTION_RE = re.compile(r"\d+[\.、]\s*([^?？]+[?？]?)")


def _normalize_question(question: str) -> str:
    compact = re.sub(r"\s+", " ", question).strip()
    numbered = NUMBERED_QUESTION_RE.findall(compact)
    if numbered:
        return numbered[0].strip()

    if compact.count("？") + compact.count("?") > 1:
        match = re.search(r"(.+?[？?])", compact)
        if match:
            return match.group(1).strip()

    return compact


def _normalize_choice_options(options: Any) -> list[str]:
    if not isinstance(options, list):
        return []
    normalized: list[str] = []
    for option in options:
        if not isinstance(option, str):
            continue
        value = option.strip()
        if value and value not in normalized:
            normalized.append(value)
    return normalized[:4]




def _normalize_fields(raw: Any) -> list[dict[str, Any]]:
    """Normalize multi-field form definitions (≤3 fields)."""
    if not isinstance(raw, list):
        return []
    valid_types = {
        "text",
        "single_choice",
        "multi_choice",
        "number",
        "date",
        "scale",
    }
    fields: list[dict[str, Any]] = []
    for item in raw[:3]:
        if not isinstance(item, dict):
            continue
        key = str(item.get("key", "")).strip()
        label = str(item.get("label", "")).strip()
        if not key or not label:
            continue
        answer_type = item.get("answer_type", "text")
        if answer_type not in valid_types:
            answer_type = "text"
        options = _normalize_choice_options(item.get("options", []))
        required = item.get("required", True)
        if not isinstance(required, bool):
            required = True
        fields.append(
            {
                "key": key,
                "label": label,
                "answer_type": answer_type,
                "options": options,
                "required": required,
            }
        )
    return fields

def _build_default_context(question: str, reason: str) -> str:
    if isinstance(reason, str) and reason.strip():
        return reason.strip()
    if YES_NO_QUESTION_RE.search(question):
        return "这能帮助我确认当前情况是否已经伴随明显不适，从而决定下一步判断重点。"
    return "为了给出更准确的下一步判断，我需要先确认这一点。"


async def handle_ask_user(arguments: dict[str, Any]) -> ToolResult:
    """Handle an ask_user tool call.

    Always returns interrupted status — the orchestration layer must
    persist the interaction and pause the run. This handler never blocks
    waiting for a user response.
    """
    question = _normalize_question(str(arguments.get("question", "")).strip())
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

    options = _normalize_choice_options(arguments.get("options", []))
    if answer_type == "text" and not options and YES_NO_QUESTION_RE.search(question):
        answer_type = "single_choice"
        options = ["是", "否"]

    allow_custom_input = arguments.get("allow_custom_input")
    if not isinstance(allow_custom_input, bool):
        allow_custom_input = bool(options)

    reason = str(arguments.get("reason", "")).strip()
    context = str(arguments.get("context", "")).strip() or _build_default_context(
        question, reason
    )
    fields = _normalize_fields(arguments.get("fields"))

    content: dict[str, Any] = {
        "question": question,
        "reason": reason,
        "answer_type": answer_type,
        "options": options,
        "allow_custom_input": allow_custom_input,
        "required": arguments.get("required", True),
        "context": context,
    }
    if fields:
        content["fields"] = fields

    # Return interrupted — the run should pause here
    return ToolResult(
        tool_call_id="",
        tool_name="ask_user",
        status=ToolStatus.INTERRUPTED,
        content=content,
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
