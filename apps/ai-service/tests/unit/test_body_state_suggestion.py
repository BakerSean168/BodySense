"""BS-AGENT-001: the runtime only *suggests*, citing the user's exact words."""

import asyncio
import json
from typing import Any
from unittest.mock import patch
from uuid import uuid4

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from pydantic_ai.messages import ModelMessage, ModelResponse, ToolCallPart
from pydantic_ai.models.function import AgentInfo, FunctionModel
from pydantic_ai.models.test import TestModel

from src.ai import GatewayUnavailableError
from src.api.routes.runtime import _e2e_intake_tool_id
from src.api.runtime_proto_adapter import RuntimeEventError, serialize_runtime_event
from src.models.body_state_suggestion import (
    BodyStateSuggestionDraft,
    BodyStateSuggestionOutput,
    body_state_suggestion_envelope,
    validate_suggestion,
)
from src.models.consultation_runtime_event import (
    ConsultationRuntimeEvent,
    InteractionRequiredRuntimeEvent,
    StreamDoneRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
)
from src.runtime import consultation_thread
from src.runtime.body_state_suggestion import (
    propose_body_state_suggestion,
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
    SUGGESTION_RETRIES,
    BodyStateSuggestionService,
)
from src.testing_support.body_state_suggestion_fixture import body_state_suggestion_fixture

MESSAGE_ID = "6f1c7a52-6d0e-4c8e-9a51-0e4c9b3f6a11"
RUN_ID = "11111111-1111-4111-8111-111111111111"


def draft(**values: Any) -> BodyStateSuggestionOutput:
    return BodyStateSuggestionOutput(suggestion=BodyStateSuggestionDraft(**values))


def test_envelope_cites_the_exact_rune_span_and_go_revision() -> None:
    text = "我右肩又疼了，我觉得是久坐导致的。"
    envelope = body_state_suggestion_envelope(
        draft(kind="hypothesis", quote="我觉得是久坐导致的"),
        text,
        message_id=MESSAGE_ID,
        base_revision=4,
    )
    assert envelope == {
        "kind": "hypothesis",
        "cited_message_id": MESSAGE_ID,
        "span": {"start": 7, "end": 16},
        "base_body_state_revision": 4,
    }
    start, end = envelope["span"]["start"], envelope["span"]["end"]  # type: ignore[index]
    assert text[start:end] == "我觉得是久坐导致的"
    # Non-BMP characters count as one rune, as in Go.
    emoji = "😣我觉得是久坐导致的。"
    span = body_state_suggestion_envelope(
        draft(kind="hypothesis", quote="我觉得是久坐导致的"),
        "😣。" + emoji[1:],
        message_id=MESSAGE_ID,
        base_revision=0,
    )["span"]  # type: ignore[index]
    assert span == {"start": 2, "end": 11}
    assert (
        body_state_suggestion_envelope(
            BodyStateSuggestionOutput(), text, message_id=MESSAGE_ID, base_revision=4
        )
        is None
    )


def test_observation_and_episode_labels_are_copied_from_the_quote() -> None:
    text = "我早上量血压是135/88 mmHg。我右肩又开始疼了。"
    observation = body_state_suggestion_envelope(
        draft(
            kind="observation",
            quote="我早上量血压是135/88 mmHg",
            observation_kind="blood_pressure",
            raw_value="135/88",
            raw_unit="mmHg",
        ),
        text,
        message_id=MESSAGE_ID,
        base_revision=1,
    )
    assert observation is not None
    assert observation["payload"] == {
        "observation_kind": "blood_pressure",
        "raw_value": "135/88",
        "raw_unit": "mmHg",
    }
    episode = body_state_suggestion_envelope(
        draft(kind="episode", quote="我右肩又开始疼了", symptom_type="疼"),
        text,
        message_id=MESSAGE_ID,
        base_revision=1,
        episode_statements=("我右肩又开始疼了",),
    )
    assert episode is not None
    assert episode["payload"] == {"symptom_type": "疼"}
    assert "fact_id" not in json.dumps(episode)


@pytest.mark.parametrize(
    ("text", "values", "statements"),
    [
        ("我觉得是久坐导致的。", {"kind": "hypothesis", "quote": "我觉得是久坐引起的"}, ()),
        ("我觉得是久坐导致的。", {"kind": "hypothesis", "quote": "觉得是久坐导致的"}, ()),
        ("我觉得是久坐导致的。", {"kind": "hypothesis", "quote": "我觉得是久坐"}, ()),
        ("我觉得累。我觉得累。", {"kind": "hypothesis", "quote": "我觉得累"}, ()),
        ("我妈妈觉得是久坐导致的。", {"kind": "hypothesis", "quote": "我妈妈觉得是久坐导致的"}, ()),
        ("我是不是久坐导致的？", {"kind": "hypothesis", "quote": "我是不是久坐导致的？"}, ()),
        (
            "我觉得是久坐导致的。",
            {"kind": "hypothesis", "quote": "我觉得是久坐导致的", "symptom_type": "久坐"},
            (),
        ),
        (
            "我早上量血压是135/88。",
            {"kind": "observation", "quote": "我早上量血压是135/88", "observation_kind": "高血压"},
            (),
        ),
        (
            "我早上量血压是135/88。",
            {
                "kind": "observation",
                "quote": "我早上量血压是135/88",
                "observation_kind": "blood_pressure",
                "raw_value": "140/90",
            },
            (),
        ),
        (
            "我早上量血压是135/88。",
            {
                "kind": "observation",
                "quote": "我早上量血压是135/88",
                "observation_kind": "blood_pressure",
                "raw_unit": "mmHg",
            },
            (),
        ),
        (
            "我右肩又开始疼了。",
            {"kind": "episode", "quote": "我右肩又开始疼了", "symptom_type": "疼"},
            (),
        ),
        (
            "我右肩又开始疼了。",
            {"kind": "episode", "quote": "我右肩又开始疼了", "symptom_type": "肩周炎"},
            ("我右肩又开始疼了",),
        ),
    ],
)
def test_unquoted_or_unowned_suggestions_are_rejected(text, values, statements) -> None:
    with pytest.raises(ValueError):
        validate_suggestion(draft(**values), text, statements)


def test_proposer_retries_a_bounded_number_of_times_then_suggests_nothing() -> None:
    calls: list[int] = []

    def invented(_: list[ModelMessage], info: AgentInfo) -> ModelResponse:
        calls.append(1)
        return ModelResponse(
            parts=[
                ToolCallPart(
                    info.output_tools[0].name,
                    {"suggestion": {"kind": "hypothesis", "quote": "我觉得是久坐引起的"}},
                )
            ]
        )

    service = BodyStateSuggestionService(model_resolver=lambda _: FunctionModel(invented))
    result = asyncio.run(
        propose_body_state_suggestion(
            {
                "latest_user_message": "我觉得是久坐导致的。",
                "latest_user_message_id": MESSAGE_ID,
                "body_state": {"records_available": True, "current_revision": 2},
            },
            manifest=get_consultation_manifest(None),
            service=service,
        )
    )
    assert result == {"body_state_suggestion": None}
    assert len(calls) == SUGGESTION_RETRIES + 1


def test_proposer_retry_can_recover_with_the_users_exact_words() -> None:
    attempts = iter(["我觉得是久坐引起的", "我觉得是久坐导致的"])

    def corrected(_: list[ModelMessage], info: AgentInfo) -> ModelResponse:
        quote = next(attempts)
        return ModelResponse(
            parts=[
                ToolCallPart(
                    info.output_tools[0].name,
                    {"suggestion": {"kind": "hypothesis", "quote": quote}},
                )
            ]
        )

    service = BodyStateSuggestionService(model_resolver=lambda _: FunctionModel(corrected))
    result = asyncio.run(
        propose_body_state_suggestion(
            {
                "latest_user_message": "我觉得是久坐导致的。",
                "latest_user_message_id": MESSAGE_ID,
                "body_state": {"records_available": True, "current_revision": 2},
                "body_state_pending_revisions": 1,
            },
            manifest=get_consultation_manifest(None),
            service=service,
        )
    )
    assert result["body_state_suggestion"]["base_body_state_revision"] == 3


@pytest.mark.parametrize(
    "state",
    [
        {"latest_user_message_id": ""},
        {"body_state": {"records_available": False, "current_revision": 2}},
        {"red_flag_result": {"has_red_flags": True}},
        {"body_state": {"records_available": True, "current_revision": 2.5}},
    ],
)
def test_proposer_fails_closed_without_a_citable_verified_context(state) -> None:
    calls: list[str] = []

    def resolver(_: Any) -> TestModel:
        calls.append("suggest")
        return TestModel(
            custom_output_args={"suggestion": {"kind": "hypothesis", "quote": "我觉得是久坐导致的"}}
        )

    base = {
        "latest_user_message": "我觉得是久坐导致的。",
        "latest_user_message_id": MESSAGE_ID,
        "body_state": {"records_available": True, "current_revision": 2},
    }
    result = asyncio.run(
        propose_body_state_suggestion(
            {**base, **state},
            manifest=get_consultation_manifest(None),
            service=BodyStateSuggestionService(model_resolver=resolver),
        )
    )
    assert result == {"body_state_suggestion": None}
    if "body_state" not in state or state["body_state"].get("records_available") is False:
        assert calls == []


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


@pytest.mark.parametrize(
    ("facts", "base"),
    [
        ([], 3),
        # A captured non-symptom fact is committed by Go in this stream first.
        ([{"kind": "lifestyle", "statement": "我每晚睡六小时", "subject": "self"}], 4),
    ],
)
def test_turn_pauses_on_one_typed_suggestion_and_resume_never_reproposes(facts, base) -> None:
    async def run():
        graph = _build_graph(InMemorySaver())
        thread_id, run_id = str(uuid4()), str(uuid4())
        text = "我每晚睡六小时。我觉得是久坐导致的。"
        service, calls = suggesting(
            {"suggestion": {"kind": "hypothesis", "quote": "我觉得是久坐导致的"}}
        )
        patches = (
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
        )
        with patches[0], patches[1], patches[2], patches[3]:
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
        required = events[-1]
        assert isinstance(required.event, InteractionRequiredRuntimeEvent)
        assert required.tool_call_id == suggestion_tool_call_id(run_id)
        assert required.event.suggestion == {
            "kind": "hypothesis",
            "cited_message_id": MESSAGE_ID,
            "span": {"start": 8, "end": 17},
            "base_body_state_revision": base,
        }
        # The opaque question carries no suggestion authority of its own.
        assert "suggestion" not in required.event.question
        assert "purpose" not in required.event.question
        ask = [e for e in events if isinstance(e.event, ToolCallRuntimeEvent)]
        assert ask[-1].event.tool == "ask_user"
        # The wire event is valid against the runtime Proto.
        wire = json.loads(serialize_runtime_event(required))
        assert wire["interaction_required"]["suggestion"]["kind"] == "hypothesis"
        assert calls == ["suggest"]

        waiting = await graph.aget_state({"configurable": {"thread_id": thread_id}})
        answer = {"selected": ["记录为我的猜测"], "text": "记录为我的猜测"}
        with patches[0], patches[1], patches[2], patches[3]:
            resumed = [
                event
                async for event in resume_thread_interrupt(
                    thread_id=thread_id,
                    conversation_id=thread_id,
                    run_id=run_id,
                    configuration_id=get_consultation_manifest(None).configuration_id,
                    answer=answer,
                    interrupt_id=waiting.interrupts[0].id,
                    body_state={"records_available": True, "current_revision": base + 1},
                )
            ]
        assert isinstance(resumed[-1].event, StreamDoneRuntimeEvent)
        answered = [
            e
            for e in resumed
            if isinstance(e.event, ToolResultRuntimeEvent) and e.event.tool == "ask_user"
        ]
        assert answered[0].tool_call_id == suggestion_tool_call_id(run_id)
        assert not any(isinstance(e.event, InteractionRequiredRuntimeEvent) for e in resumed)
        assert calls == ["suggest"]  # the proposer is not re-run on resume
        finished = await graph.aget_state({"configurable": {"thread_id": thread_id}})
        assert finished.values["body_state_suggestion"] is None

    asyncio.run(run())


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
