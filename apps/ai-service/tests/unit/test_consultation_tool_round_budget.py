"""The runaway model/tool guard must not count answered user questions as tool loops."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch
from uuid import uuid4

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from pydantic_ai.models.test import TestModel

from src.ai.types import AiTextDeltaEvent, AiToolCallDoneEvent
from src.models.consultation_runtime_event import (
    InteractionRequiredRuntimeEvent,
    StreamDoneRuntimeEvent,
    StreamErrorRuntimeEvent,
)
from src.runtime import consultation_thread
from src.runtime.consultation_thread import (
    MAX_TOOL_ROUNDS,
    _build_graph,
    get_consultation_manifest,
    resume_thread_interrupt,
    stream_thread_turn,
)
from src.services.body_fact_intake_service import BodyFactIntakeService


class ScriptedModel:
    def __init__(self, *, tool_name: str, questions: int | None) -> None:
        self.tool_name = tool_name
        self.questions = questions
        self.calls = 0

    async def generate_stream(self, _request):
        self.calls += 1
        if self.questions is not None and self.calls > self.questions:
            yield AiTextDeltaEvent(text="问诊结束。")
            return
        yield AiToolCallDoneEvent(
            tool_call_id=f"budget-call-{self.calls}",
            tool_name=self.tool_name,
            tool_arguments=(
                {"question": f"第 {self.calls} 个问题？", "answer_type": "text"}
                if self.tool_name == "ask_user"
                else {}
            ),
        )


def intake_without_facts() -> BodyFactIntakeService:
    return BodyFactIntakeService(
        model_resolver=lambda _: TestModel(custom_output_args={"turn_kind": "other", "facts": []})
    )


@pytest.mark.asyncio
async def test_same_run_survives_more_than_six_answered_questions() -> None:
    """Each user answer starts another autonomous segment, not another Run."""
    graph = _build_graph(InMemorySaver())
    thread_id, run_id = str(uuid4()), str(uuid4())
    manifest = get_consultation_manifest(None)
    question_count = MAX_TOOL_ROUNDS + 1
    model = ScriptedModel(tool_name="ask_user", questions=question_count)
    config = {"configurable": {"thread_id": thread_id}}

    with (
        patch.object(consultation_thread, "get_runtime_graph", return_value=graph),
        patch(
            "src.runtime.body_fact_capture.BodyFactIntakeService",
            return_value=intake_without_facts(),
        ),
        patch.object(
            consultation_thread,
            "propose_body_state_suggestion",
            new=AsyncMock(return_value={}),
        ),
        patch.object(consultation_thread, "_get_ai_service", return_value=model),
    ):
        events = [
            event
            async for event in stream_thread_turn(
                thread_id=thread_id,
                conversation_id=thread_id,
                run_id=run_id,
                user_id=str(uuid4()),
                user_message="开始问诊",
                profile={},
                extracted_info=[],
                phase="collecting",
                body_state={"records_available": True, "current_revision": 0},
            )
        ]

        for i in range(question_count):
            required = events[-1]
            assert isinstance(required.event, InteractionRequiredRuntimeEvent)
            waiting = await graph.aget_state(config)
            assert len(waiting.interrupts) == 1
            assert required.event.interaction_id == waiting.interrupts[0].id
            events = [
                event
                async for event in resume_thread_interrupt(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    configuration_id=manifest.configuration_id,
                    answer={"text": f"我的回答 {i + 1}"},
                    interrupt_id=waiting.interrupts[0].id,
                    body_state={"records_available": True, "current_revision": 0},
                )
            ]
            assert all(event.run_id == run_id for event in events)
            assert not any(isinstance(event.event, StreamErrorRuntimeEvent) for event in events)

    assert model.calls == question_count + 1
    assert isinstance(events[-1].event, StreamDoneRuntimeEvent)
    finished = await graph.aget_state(config)
    assert finished.interrupts == ()
    assert finished.values["run_id"] == run_id
    assert finished.values["tool_rounds"] < MAX_TOOL_ROUNDS


@pytest.mark.asyncio
async def test_unattended_tool_loop_still_stops_after_six_model_calls() -> None:
    graph = _build_graph(InMemorySaver())
    model = ScriptedModel(tool_name="invalid_repeated_tool", questions=None)
    thread_id, run_id = str(uuid4()), str(uuid4())

    with (
        patch.object(consultation_thread, "get_runtime_graph", return_value=graph),
        patch(
            "src.runtime.body_fact_capture.BodyFactIntakeService",
            return_value=intake_without_facts(),
        ),
        patch.object(
            consultation_thread,
            "propose_body_state_suggestion",
            new=AsyncMock(return_value={}),
        ),
        patch.object(consultation_thread, "_get_ai_service", return_value=model),
    ):
        events = [
            event
            async for event in stream_thread_turn(
                thread_id=thread_id,
                conversation_id=thread_id,
                run_id=run_id,
                user_id=str(uuid4()),
                user_message="开始问诊",
                profile={},
                extracted_info=[],
                phase="collecting",
                body_state={"records_available": True, "current_revision": 0},
            )
        ]

    assert model.calls == MAX_TOOL_ROUNDS
    assert any(
        isinstance(event.event, StreamErrorRuntimeEvent)
        and event.event.message == "模型工具循环超过上限"
        for event in events
    )
    assert not any(isinstance(event.event, InteractionRequiredRuntimeEvent) for event in events)
