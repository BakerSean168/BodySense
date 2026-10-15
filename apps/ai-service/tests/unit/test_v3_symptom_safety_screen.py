"""v3 runtime: a proposed symptom pauses on the bound safety screen (native interrupt)."""

import asyncio
from typing import Any
from unittest.mock import Mock, patch
from uuid import uuid4

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from pydantic_ai.models.test import TestModel

from src.ai import GatewayUnavailableError
from src.models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    InteractionRequiredRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamDoneRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
)
from src.runtime import consultation_thread
from src.runtime.consultation_thread import (
    _build_graph,
    get_consultation_manifest,
    resume_thread_interrupt,
    stream_thread_turn,
)
from src.services.body_fact_intake_service import BodyFactIntakeService
from src.services.consultation_state_acquisition import safety_screen_capture_id
from src.services.red_flag_detector import get_red_flag_detector

SAFETY_LABEL = "请选出目前出现的所有安全信号（可多选）；若五项都没有，请只选“以上均无”。"


def counting_intake(facts: list[dict[str, Any]], turn_kind: str = "self_report"):
    calls: list[str] = []

    def resolver(_: Any) -> TestModel:
        calls.append("assess")
        return TestModel(custom_output_args={"turn_kind": turn_kind, "facts": facts})

    return BodyFactIntakeService(model_resolver=resolver), calls


async def run_turn(graph, *, thread_id: str, run_id: str, text: str, facts: list[dict[str, Any]]):
    service, calls = counting_intake(facts, "self_report" if facts else "other")
    with (
        patch.object(consultation_thread, "get_runtime_graph", return_value=graph),
        patch("src.runtime.body_fact_capture.BodyFactIntakeService", return_value=service),
        patch.object(
            consultation_thread, "_get_ai_service", side_effect=GatewayUnavailableError("offline")
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
                profile={},
                extracted_info=[],
                phase="collecting",
                body_state={"records_available": True, "current_revision": 3},
            )
        ]
    return events, calls


@pytest.mark.parametrize(
    ("selection", "expect_red_flag"), [(["以上均无"], False), (["麻木", "无力"], None)]
)
def test_symptom_turn_interrupts_on_bound_safety_screen_and_resumes(selection, expect_red_flag):
    async def run():
        graph = _build_graph(InMemorySaver())
        thread_id, run_id = str(uuid4()), str(uuid4())
        text = "我昨天肩膀不舒服。"
        events, calls = await run_turn(
            graph,
            thread_id=thread_id,
            run_id=run_id,
            text=text,
            facts=[
                {"kind": "lifestyle", "statement": text, "subject": "self"},
                {"kind": "symptom", "statement": text, "subject": "self", "presence": "present"},
            ],
        )
        assert isinstance(events[0].event, AgentConfigurationRuntimeEvent)
        proposal_index = next(
            i
            for i, event in enumerate(events)
            if isinstance(event.event, ToolResultRuntimeEvent)
            and event.event.tool == "record_body_state"
        )
        proposal = events[proposal_index]
        fact_tool_call = proposal.tool_call_id
        assert proposal.event.result["status"] == "proposed"
        assert [fact["kind"] for fact in proposal.event.result["facts"]] == ["lifestyle", "symptom"]

        # The run stops on a native interrupt for the safety screen of the
        # proposed symptom (fact index 1), after the proposal reached Go.
        required = events[-1]
        assert isinstance(required.event, InteractionRequiredRuntimeEvent)
        question = required.event.question
        capture = safety_screen_capture_id(fact_tool_call, 1)
        assert required.tool_call_id == f"intake-{capture}"
        assert question["purpose"] == "symptom_intake"
        assert question["state_binding"] == {
            "revision": "symptom-intake-binding-v2",
            "capture_id": capture,
            "fact_ref": {"tool_call_id": fact_tool_call, "index": 1},
            "field_map": {"safety_signals": "safety_signals"},
        }
        assert question["fields"][0]["options"][-1] == "以上均无"
        assert question["fields"][0]["exclusive_options"] == ["以上均无"]
        ask_index = next(
            i
            for i, event in enumerate(events)
            if isinstance(event.event, ToolCallRuntimeEvent) and event.event.tool == "ask_user"
        )
        assert proposal_index < ask_index < len(events) - 1
        # No advice is streamed before the screen is answered.
        assert not any(isinstance(event.event, TextDeltaRuntimeEvent) for event in events)
        assert not any(isinstance(event.event, StreamDoneRuntimeEvent) for event in events)
        waiting = await graph.aget_state({"configurable": {"thread_id": thread_id}})
        assert len(waiting.interrupts) == 1
        native_id = waiting.interrupts[0].id
        assert required.event.interaction_id == native_id

        answer_text = f"{SAFETY_LABEL}: {'，'.join(selection)}"
        answer = {"fields": {"safety_signals": selection}, "text": answer_text}
        detector = Mock(wraps=get_red_flag_detector())
        resume_service, resume_calls = counting_intake([], "other")
        with (
            patch.object(consultation_thread, "get_runtime_graph", return_value=graph),
            patch.object(consultation_thread, "get_red_flag_detector", return_value=detector),
            patch(
                "src.runtime.body_fact_capture.BodyFactIntakeService", return_value=resume_service
            ),
            patch.object(
                consultation_thread,
                "_get_ai_service",
                side_effect=GatewayUnavailableError("offline"),
            ),
        ):
            resumed = [
                event
                async for event in resume_thread_interrupt(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    configuration_id=get_consultation_manifest(None).configuration_id,
                    answer=answer,
                    interrupt_id=native_id,
                    body_state={"records_available": True, "current_revision": 5},
                )
            ]
        assert isinstance(resumed[0].event, AgentConfigurationRuntimeEvent)
        assert isinstance(resumed[-1].event, StreamDoneRuntimeEvent)
        assert all(event.run_id == run_id for event in resumed)
        # The checklist answer is never re-extracted as a statement (Go records
        # it as the screen of the committed symptom), but the red-flag screen
        # still runs on the accepted answer text.
        assert calls == ["assess"]
        assert resume_calls == []
        assert not any(
            isinstance(event.event, ToolResultRuntimeEvent)
            and event.event.tool == "record_body_state"
            for event in resumed
        )
        assert detector.detect.call_args_list[0].args == ([], answer_text)
        answered = [
            event
            for event in resumed
            if isinstance(event.event, ToolResultRuntimeEvent) and event.event.tool == "ask_user"
        ]
        assert len(answered) == 1
        assert answered[0].tool_call_id == f"intake-{capture}"
        assert answered[0].event.result == {"answer": answer}
        if expect_red_flag is False:
            assert not any(isinstance(e.event, RedFlagDetectedRuntimeEvent) for e in resumed)
        finished = await graph.aget_state({"configurable": {"thread_id": thread_id}})
        assert finished.interrupts == ()

    asyncio.run(run())


@pytest.mark.parametrize(
    "facts",
    [
        [],
        [{"kind": "symptom", "statement": "我没有肩痛。", "subject": "self", "presence": "absent"}],
        [{"kind": "lifestyle", "statement": "我每晚睡七小时。", "subject": "self"}],
    ],
)
def test_turn_without_non_absent_symptom_asks_no_safety_screen(facts):
    async def run():
        graph = _build_graph(InMemorySaver())
        thread_id = str(uuid4())
        text = facts[0]["statement"] if facts else "今天天气怎么样"
        events, _ = await run_turn(
            graph, thread_id=thread_id, run_id=str(uuid4()), text=text, facts=facts
        )
        assert not any(isinstance(e.event, InteractionRequiredRuntimeEvent) for e in events)
        assert isinstance(events[-1].event, StreamDoneRuntimeEvent)

    asyncio.run(run())


def test_model_authored_ask_user_cannot_carry_an_intake_binding():
    async def run():
        captured: dict[str, Any] = {}

        async def execute(tool_call_id, tool_name, arguments):
            captured.update(arguments)
            from src.services.agent.tools.ask_user import handle_ask_user

            return await handle_ask_user(arguments)

        executor = Mock()
        executor.execute = execute
        writer_events: list[dict[str, Any]] = []
        state = {
            "pending_tool_calls": [
                {
                    "id": "call_model_1",
                    "name": "ask_user",
                    "arguments": {
                        "question": "有没有麻木？",
                        "purpose": "symptom_intake",
                        "state_binding": {
                            "capture_id": "0" * 24,
                            "fact_ref": {"tool_call_id": "bodyfact-intake-v1:x", "index": 0},
                        },
                    },
                }
            ],
            "consultation_manifest": get_consultation_manifest(None),
        }
        with (
            patch.object(consultation_thread, "get_consultation_executor", return_value=executor),
            patch.object(consultation_thread, "interrupt", side_effect=lambda payload: payload),
        ):
            await consultation_thread.execute_tool(state, writer=writer_events.append)
        assert "purpose" not in captured and "state_binding" not in captured

    asyncio.run(run())
