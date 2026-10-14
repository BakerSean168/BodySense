import asyncio
from types import SimpleNamespace
from typing import Any, TypedDict
from unittest.mock import AsyncMock, Mock, patch
from uuid import uuid4

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.config import get_stream_writer
from langgraph.graph import END, START, StateGraph
from langgraph.types import interrupt
from pydantic_ai.models.test import TestModel

from src.models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamDoneRuntimeEvent,
    ToolResultRuntimeEvent,
)
from src.runtime.body_fact_capture import acquire_body_fact_answer
from src.runtime.consultation_thread import get_consultation_manifest, resume_thread_interrupt
from src.services.body_fact_intake_service import BodyFactIntakeService


class ResumeState(TypedDict, total=False):
    run_id: str
    consultation_manifest: dict[str, Any]
    body_state: dict[str, Any]
    intake_result: dict[str, Any]
    answer: dict[str, Any]
    red_flag_result: dict[str, Any]


def intake_for(text: str, kind: str = "self_report") -> BodyFactIntakeService:
    output: dict[str, Any] = {"turn_kind": kind, "facts": []}
    if kind == "self_report":
        output["facts"] = [
            {"kind": "symptom", "statement": text, "subject": "self", "presence": "absent"}
        ]
    return BodyFactIntakeService(model_resolver=lambda _: TestModel(custom_output_args=output))


@pytest.mark.parametrize("new_risk", [False, True])
def test_real_checkpoint_resumes_original_run_and_emits_only_a_proposal(new_risk):
    async def run():
        run_id, thread_id = str(uuid4()), str(uuid4())
        manifest = get_consultation_manifest(None)
        text = "我没有肩痛。"

        async def wait_for_answer(state: ResumeState):
            answer = interrupt({"question": "Describe yourself.", "tool_call_id": "ask-1"})
            get_stream_writer()({"type": "text_delta", "delta": "Answer received."})
            return {"answer": answer}

        builder = StateGraph(ResumeState)
        builder.add_node("wait", wait_for_answer)
        builder.add_edge(START, "wait")
        builder.add_edge("wait", END)
        graph = builder.compile(checkpointer=InMemorySaver())
        config = {"configurable": {"thread_id": thread_id}}
        await graph.ainvoke(
            {
                "run_id": run_id,
                "consultation_manifest": manifest.model_dump(mode="json"),
                "red_flag_result": {"has_red_flags": True, "flags": [{"category": "earlier"}]},
            },
            config=config,
        )
        waiting = await graph.aget_state(config)
        assert len(waiting.interrupts) == 1
        native_id = waiting.interrupts[0].id
        detector = Mock()
        detection = SimpleNamespace(
            has_red_flags=new_risk,
            to_dict=lambda: {
                "has_red_flags": new_risk,
                "flags": [
                    {"category": "new", "severity": "warning", "message": "synthetic screen"}
                ],
            },
        )
        detector.detect.return_value = detection
        with (
            patch("src.runtime.consultation_thread.get_red_flag_detector", return_value=detector),
            patch("src.runtime.consultation_thread.get_runtime_graph", return_value=graph),
            patch(
                "src.runtime.body_fact_capture.BodyFactIntakeService", return_value=intake_for(text)
            ),
        ):
            events = [
                event
                async for event in resume_thread_interrupt(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    configuration_id=manifest.configuration_id,
                    answer={"text": text},
                    interrupt_id=native_id,
                    body_state={"records_available": True, "current_revision": 5},
                )
            ]
        assert isinstance(events[0].event, AgentConfigurationRuntimeEvent)
        assert isinstance(events[-1].event, StreamDoneRuntimeEvent)
        assert all(event.run_id == run_id for event in events)
        proposals = [event for event in events if isinstance(event.event, ToolResultRuntimeEvent)]
        assert len(proposals) == 1
        result = proposals[0].event.result
        assert result["status"] == "proposed"
        assert result["expected_revision"] == 5
        assert result["facts"][0]["details"]["presence"] == "absent"
        assert result["facts"][0]["statement"] == text
        assert "receipt" not in result
        assert native_id in proposals[0].tool_call_id
        finished = await graph.aget_state(config)
        assert finished.interrupts == ()
        assert finished.values["run_id"] == run_id
        assert finished.values["answer"] == {"text": text}
        assert finished.values["intake_result"]["capture_status"] == "proposed"
        detector.detect.assert_called_once_with([], text)
        risk_events = [
            i
            for i, event in enumerate(events)
            if isinstance(event.event, RedFlagDetectedRuntimeEvent)
        ]
        proposal_index = next(
            i for i, event in enumerate(events) if isinstance(event.event, ToolResultRuntimeEvent)
        )
        if new_risk:
            assert risk_events and risk_events[0] < proposal_index
            assert finished.values["red_flag_result"]["flags"][0]["category"] == "new"
        else:
            assert risk_events == []
            assert finished.values["red_flag_result"]["flags"][0]["category"] == "earlier"

    asyncio.run(run())


@pytest.mark.parametrize(
    "violation", ["wrong_run", "wrong_interrupt", "no_interrupt", "missing_id"]
)
def test_wrong_resume_identity_never_calls_model_or_mutates_checkpoint(violation):
    async def run():
        run_id, native_id = str(uuid4()), uuid4().hex
        manifest = get_consultation_manifest(None)
        state = SimpleNamespace(
            values={"run_id": run_id, "consultation_manifest": manifest},
            interrupts=[SimpleNamespace(id=native_id)],
        )
        requested_run, requested_interrupt = run_id, native_id
        if violation == "wrong_run":
            requested_run = str(uuid4())
        elif violation == "wrong_interrupt":
            requested_interrupt = uuid4().hex
        elif violation == "no_interrupt":
            state.interrupts = []
        else:
            requested_interrupt = None
        graph = SimpleNamespace(aget_state=AsyncMock(return_value=state), astream=AsyncMock())
        extract = AsyncMock()
        with (
            patch("src.runtime.consultation_thread.get_runtime_graph", return_value=graph),
            patch("src.runtime.consultation_thread.acquire_body_fact_answer", extract),
            pytest.raises(ValueError),
        ):
            async for _ in resume_thread_interrupt(
                thread_id=str(uuid4()),
                conversation_id=str(uuid4()),
                run_id=requested_run,
                configuration_id=manifest.configuration_id,
                answer={"text": "我没有肩痛。"},
                interrupt_id=requested_interrupt,
            ):
                pass
        extract.assert_not_called()
        graph.astream.assert_not_called()

    asyncio.run(run())


def test_two_answers_on_same_run_get_distinct_stable_tool_ids():
    async def run():
        run_id, a, b = str(uuid4()), str(uuid4()), str(uuid4())
        identities = []
        for native_id in (a, b, a):
            events = []
            await acquire_body_fact_answer(
                run_id=run_id,
                interrupt_id=native_id,
                answer={"text": "我没有肩痛。"},
                body_state={"records_available": True, "current_revision": 0},
                writer=events.append,
                manifest=get_consultation_manifest(None),
                intake_service=intake_for("我没有肩痛。"),
            )
            identities.append(events[0]["id"])
        assert identities[0] == identities[2]
        assert identities[0] != identities[1]

    asyncio.run(run())


@pytest.mark.parametrize("answer", [{"text": "yes"}, {"text": "left"}, {"value": 7}])
def test_short_contextual_answers_do_not_promote_the_question_to_a_fact(answer):
    async def run():
        events = []
        result = await acquire_body_fact_answer(
            run_id=str(uuid4()),
            interrupt_id=str(uuid4()),
            answer=answer,
            body_state={"records_available": True, "current_revision": 0},
            writer=events.append,
            manifest=get_consultation_manifest(None),
            intake_service=intake_for("", kind="other"),
        )
        assert result["capture_status"] == "not_applicable"
        assert not any(item["type"] == "tool_result" for item in events)

    asyncio.run(run())
