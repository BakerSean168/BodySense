"""Structured no-side-effect tool for answer claim ↔ published evidence attribution."""

from __future__ import annotations

from typing import Any

from ..tool_types import RuntimeToolDefinition, ToolCategory

RECORD_ANSWER_ATTRIBUTION_SCHEMA: dict[str, Any] = {
    "name": "record_answer_attribution",
    "description": (
        "当你准备基于 search_knowledge 返回的 Published Evidence Ref 向用户陈述实质性健康知识时，"
        "先用此工具记录每条简短结论与它实际使用的 Evidence Ref。"
        "只能使用本轮搜索结果中明确给出的 Evidence Ref，不得猜测或编造。"
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "claims": {
                "type": "array",
                "minItems": 1,
                "maxItems": 6,
                "items": {
                    "type": "object",
                    "properties": {
                        "claim_text": {
                            "type": "string",
                            "description": "准备在最终回答中表达的一条简短、可核验事实性结论",
                        },
                        "evidence_refs": {
                            "type": "array",
                            "minItems": 1,
                            "maxItems": 3,
                            "items": {"type": "string"},
                            "description": "支持该结论的 Published Evidence Ref 列表",
                        },
                    },
                    "required": ["claim_text", "evidence_refs"],
                    "additionalProperties": False,
                },
            }
        },
        "required": ["claims"],
        "additionalProperties": False,
    },
}


async def handle_record_answer_attribution(arguments: dict[str, Any]) -> dict[str, Any]:
    """Return arguments; turn-scoped identity validation belongs to the runtime."""
    return {"claims": arguments.get("claims", [])}


def make_record_answer_attribution_tool() -> RuntimeToolDefinition:
    return RuntimeToolDefinition(
        name=RECORD_ANSWER_ATTRIBUTION_SCHEMA["name"],
        description=RECORD_ANSWER_ATTRIBUTION_SCHEMA["description"],
        parameters=RECORD_ANSWER_ATTRIBUTION_SCHEMA["parameters"],
        category=ToolCategory.QUERY,
        handler=handle_record_answer_attribution,
        required_params=["claims"],
    )
