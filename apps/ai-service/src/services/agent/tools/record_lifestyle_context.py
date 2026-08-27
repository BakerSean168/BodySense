"""record_lifestyle_context tool — normalizes explicit user-reported lifestyle state."""

from __future__ import annotations

from typing import Any

from ....prompts.consultation import LIFESTYLE_CONTEXT_TOOL
from ..tool_types import RuntimeToolDefinition, ToolCategory


async def handle_record_lifestyle_context(arguments: dict[str, Any]) -> dict[str, Any]:
    section = str(arguments.get("section") or "").strip()
    summary = str(arguments.get("summary") or "").strip()
    allowed = {"activity", "sleep", "exercise", "nutrition", "substances", "recovery"}
    if section not in allowed:
        return {"error": "unsupported lifestyle section"}
    if not summary:
        return {"error": "summary is required"}
    details = arguments.get("details")
    if not isinstance(details, dict):
        details = {}
    return {"section": section, "summary": summary, "details": details}


def make_record_lifestyle_context_tool() -> RuntimeToolDefinition:
    return RuntimeToolDefinition(
        name=LIFESTYLE_CONTEXT_TOOL["name"],
        description=LIFESTYLE_CONTEXT_TOOL["description"],
        parameters=LIFESTYLE_CONTEXT_TOOL["parameters"],
        category=ToolCategory.QUERY,
        handler=handle_record_lifestyle_context,
        required_params=["section", "summary"],
    )
