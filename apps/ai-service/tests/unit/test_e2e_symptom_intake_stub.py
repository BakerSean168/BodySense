"""The deterministic E2E intake must be the production safety screen Go binds."""

import pytest

from src.api.routes.runtime import _e2e_intake_tool_id, _e2e_safety_screen_question
from src.runtime.consultation_thread import get_consultation_manifest
from src.services.consultation_state_acquisition import safety_screen_capture_id
from src.testing_support.body_fact_fixture import body_fact_fixture_events

RUN_ID = "11111111-1111-4111-8111-111111111111"
BODY_STATE = {"records_available": True, "current_revision": 0}


@pytest.mark.asyncio
async def test_e2e_symptom_turn_asks_the_bound_safety_screen() -> None:
    events, question = await body_fact_fixture_events(
        "我昨天肩膀不舒服。", BODY_STATE, get_consultation_manifest(None), RUN_ID
    )
    proposal = next(item for item in events if item["type"] == "tool_result")
    assert proposal["tool"] == "record_body_state"
    assert proposal["result"]["facts"][0]["details"] == {"presence": "present"}
    assert question is not None

    tool_id, normalized = await _e2e_safety_screen_question(question)
    binding = normalized["state_binding"]
    assert normalized["purpose"] == "symptom_intake"
    assert binding["revision"] == "symptom-intake-binding-v2"
    assert binding["fact_ref"] == {"tool_call_id": proposal["id"], "index": 0}
    assert binding["capture_id"] == safety_screen_capture_id(proposal["id"], 0)
    assert "seed_info" not in binding
    assert tool_id == f"intake-{binding['capture_id']}"
    assert [field["key"] for field in normalized["fields"]] == ["safety_signals"]
    safety = normalized["fields"][0]
    assert safety["options"] == ["外伤或创伤", "放射痛", "麻木", "无力", "头晕", "以上均无"]
    assert safety["exclusive_options"] == ["以上均无"]
    assert _e2e_intake_tool_id(RUN_ID, {"fields": {"safety_signals": ["以上均无"]}}) == tool_id
    assert _e2e_intake_tool_id(RUN_ID, {"text": "x"}) == f"ask-bodyfact:{RUN_ID}"


@pytest.mark.asyncio
@pytest.mark.parametrize("text", ["我没有肩痛。", "我每晚睡七小时。", "unrelated"])
async def test_e2e_turn_without_present_symptom_asks_nothing(text: str) -> None:
    _, question = await body_fact_fixture_events(
        text, BODY_STATE, get_consultation_manifest(None), RUN_ID
    )
    assert question is None
