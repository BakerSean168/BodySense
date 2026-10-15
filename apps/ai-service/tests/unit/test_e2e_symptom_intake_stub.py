"""The deterministic E2E intake must carry the exact runtime binding Go accepts."""

import pytest

from src.api.routes.runtime import (
    E2E_SYMPTOM_INTAKE,
    _e2e_intake_tool_id,
    _e2e_symptom_intake_question,
)

RUN_ID = "11111111-1111-4111-8111-111111111111"


@pytest.mark.asyncio
async def test_e2e_symptom_intake_is_runtime_bound() -> None:
    tool_id, question = await _e2e_symptom_intake_question(RUN_ID, E2E_SYMPTOM_INTAKE)
    binding = question["state_binding"]
    assert question["purpose"] == "symptom_intake"
    assert binding["revision"] == "symptom-intake-binding-v1"
    assert tool_id == f"intake-{binding['capture_id']}"
    assert [field["key"] for field in question["fields"]] == ["safety_signals", "duration"]
    safety = question["fields"][0]
    assert safety["options"] == ["外伤或创伤", "放射痛", "麻木", "无力", "头晕", "以上均无"]
    assert safety["exclusive_options"] == ["以上均无"]
    assert binding["field_map"]["safety_signals"] == "safety_signals"
    assert _e2e_intake_tool_id(RUN_ID, {"fields": {"duration": "2–7天"}}) == tool_id
    assert _e2e_intake_tool_id(RUN_ID, {"text": "x"}) == f"ask-bodyfact:{RUN_ID}"
