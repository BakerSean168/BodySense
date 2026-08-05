"""extract_symptom_info tool — extracts structured symptom data from conversation."""

from __future__ import annotations

from typing import Any

from ....prompts.consultation import SYMPTOM_EXTRACTION_TOOL
from ..tool_types import RuntimeToolDefinition, ToolCategory

# Re-export the schema from prompts for single source of truth
EXTRACT_SYMPTOM_SCHEMA = SYMPTOM_EXTRACTION_TOOL


async def handle_extract_symptom_info(arguments: dict[str, Any]) -> dict[str, Any]:
    """Validate and normalize extracted symptom info.

    Returns the normalized arguments dict. The active runtime graph
    (runtime/consultation_thread.py) is responsible for:
    - Per-response dedupe by body_part
    - Emitting extracted_info SSE events
    - Creating tool result messages for the LLM
    - Phase calculation
    """
    body_part = arguments.get("body_part", "").strip()
    if not body_part:
        return {"error": "body_part is required"}

    # Normalize: return only known fields with string values
    known_fields = [
        "body_part",
        "symptom_type",
        "duration",
        "trigger",
        "relief",
        "severity",
        "additional_notes",
    ]
    normalized: dict[str, Any] = {}
    for field in known_fields:
        value = arguments.get(field)
        if value is not None:
            normalized[field] = str(value).strip() if isinstance(value, str) else value

    # Ensure body_part is present
    normalized.setdefault("body_part", body_part)

    return normalized


def make_extract_symptom_info_tool() -> RuntimeToolDefinition:
    """Create a RuntimeToolDefinition for extract_symptom_info."""
    return RuntimeToolDefinition(
        name=EXTRACT_SYMPTOM_SCHEMA["name"],
        description=EXTRACT_SYMPTOM_SCHEMA["description"],
        parameters=EXTRACT_SYMPTOM_SCHEMA["parameters"],
        category=ToolCategory.QUERY,  # Not a DB write — emits structured state
        handler=handle_extract_symptom_info,
        required_params=["body_part"],
    )
