"""BS-AGENT-001: the runtime only *suggests*, citing the user's exact words."""

import asyncio
import json
from typing import Any
from unittest.mock import patch
from uuid import uuid4

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from pydantic_ai.models.test import TestModel

from src.ai import GatewayUnavailableError
from src.api.routes.runtime import _e2e_intake_tool_id
from src.api.runtime_proto_adapter import RuntimeEventError, serialize_runtime_event
from src.models.consultation_runtime_event import (
    ConsultationRuntimeEvent,
    InteractionRequiredRuntimeEvent,
)
from src.runtime import consultation_thread
from src.runtime.body_state_suggestion import (
    suggestion_tool_call_id,
)
from src.runtime.consultation_thread import (
    _build_graph,
    get_consultation_manifest,
    resume_thread_interrupt,
    stream_thread_turn,
)
from src.services.body_fact_intake_service import BodyFactIntakeService
from src.services.body_state_suggestion_service import (
    BodyStateSuggestionService,
)
from src.testing_support.body_state_suggestion_fixture import body_state_suggestion_fixture

MESSAGE_ID = "6f1c7a52-6d0e-4c8e-9a51-0e4c9b3f6a11"


RUN_ID = "11111111-1111-4111-8111-111111111111"


def suggesting(output: dict[str, Any]):
    calls: list[str] = []

    def resolver(_: Any) -> TestModel:
        calls.append("suggest")
        return TestModel(custom_output_args=output)

    return BodyStateSuggestionService(model_resolver=resolver), calls


def facts_intake(facts: list[dict[str, Any]]):
    return BodyFactIntakeService(
        model_resolver=lambda _: TestModel(
            custom_output_args={"turn_kind": "self_report" if facts else "other", "facts": facts}
        )
    )


def test_episode_may_only_quote_a_present_symptom_captured_this_turn() -> None:
    async def run():
        graph = _build_graph(InMemorySaver())
        thread_id, run_id = str(uuid4()), str(uuid4())
        text = "我右肩又开始疼了。"
        service, calls = suggesting(
            {"suggestion": {"kind": "episode", "quote": "我右肩又开始疼了。", "symptom_type": "疼"}}
        )
        facts = [{"kind": "symptom", "statement": text, "subject": "self", "presence": "present"}]
        with (
            patch.object(consultation_thread, "get_runtime_graph", return_value=graph),
            patch(
                "src.runtime.body_fact_capture.BodyFactIntakeService",
                return_value=facts_intake(facts),
            ),
            patch(
                "src.runtime.body_state_suggestion.BodyStateSuggestionService", return_value=service
            ),
            patch.object(
                consultation_thread, "_get_ai_service", side_effect=GatewayUnavailableError("off")
            ),
        ):
            events = [
                event
                async for event in stream_thread_turn(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    user_id=str(uuid4()),
                    user_message=text,
                    user_message_id=MESSAGE_ID,
                    profile={},
                    extracted_info=[],
                    phase="collecting",
                    body_state={"records_available": True, "current_revision": 3},
                )
            ]
            # The safety screen of the symptom comes first; the suggestion is
            # proposed only after it, against the refreshed BodyState.
            assert events[-1].event.question["purpose"] == "symptom_intake"
            assert calls == []
            waiting = await graph.aget_state({"configurable": {"thread_id": thread_id}})
            resumed = [
                event
                async for event in resume_thread_interrupt(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    configuration_id=get_consultation_manifest(None).configuration_id,
                    answer={"fields": {"safety_signals": ["以上均无"]}, "text": "以上均无"},
                    interrupt_id=waiting.interrupts[0].id,
                    body_state={"records_available": True, "current_revision": 5},
                )
            ]
        required = resumed[-1]
        assert isinstance(required.event, InteractionRequiredRuntimeEvent)
        assert required.event.suggestion == {
            "kind": "episode",
            "cited_message_id": MESSAGE_ID,
            "span": {"start": 0, "end": 9},
            "base_body_state_revision": 5,
            "payload": {"symptom_type": "疼"},
        }
        assert calls == ["suggest"]

    asyncio.run(run())


def test_runtime_proto_rejects_an_untyped_or_unknown_suggestion() -> None:
    def event(suggestion: dict[str, Any]) -> ConsultationRuntimeEvent:
        return ConsultationRuntimeEvent(
            seq=1,
            conversation_id=str(uuid4()),
            run_id=RUN_ID,
            event=InteractionRequiredRuntimeEvent(
                interaction_id=uuid4().hex, question={"question": "q"}, suggestion=suggestion
            ),
        )

    valid = {
        "kind": "observation",
        "cited_message_id": MESSAGE_ID,
        "span": {"start": 0, "end": 3},
        "base_body_state_revision": 1,
        "payload": {"observation_kind": "heart_rate"},
    }
    assert json.loads(serialize_runtime_event(event(valid)))["interaction_required"]["suggestion"]
    for broken in (
        {**valid, "kind": "diagnosis"},
        {**valid, "cited_message_id": "not-a-uuid"},
        {**valid, "span": {"start": 3, "end": 3}},
        {**valid, "receipt": "forged"},
    ):
        with pytest.raises(RuntimeEventError):
            serialize_runtime_event(event(broken))


def test_e2e_stub_uses_the_production_proposer() -> None:
    body_state = {"records_available": True, "current_revision": 0}
    manifest = get_consultation_manifest(None)
    hypothesis = asyncio.run(
        body_state_suggestion_fixture(
            "我觉得是久坐导致的。", body_state, manifest, RUN_ID, MESSAGE_ID
        )
    )
    assert hypothesis == {
        "kind": "hypothesis",
        "cited_message_id": MESSAGE_ID,
        "span": {"start": 0, "end": 9},
        "base_body_state_revision": 0,
    }
    assert (
        asyncio.run(
            body_state_suggestion_fixture("unrelated", body_state, manifest, RUN_ID, MESSAGE_ID)
        )
        is None
    )
    assert (
        asyncio.run(
            body_state_suggestion_fixture(
                "我觉得是久坐导致的。", body_state, manifest, RUN_ID, None
            )
        )
        is None
    )
    for option in ("记录为我的猜测", "不记录", "知道了"):
        assert _e2e_intake_tool_id(RUN_ID, {"selected": [option]}) == suggestion_tool_call_id(
            RUN_ID
        )
    assert _e2e_intake_tool_id(RUN_ID, {"selected": ["left"]}) == f"ask-bodyfact:{RUN_ID}"
