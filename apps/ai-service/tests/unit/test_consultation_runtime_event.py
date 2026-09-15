from __future__ import annotations

from src.models.consultation_runtime_event import (
    ConsultationRuntimeEventFactory,
    StreamDoneRuntimeEvent,
    TextDeltaRuntimeEvent,
)


def test_runtime_event_factory_owns_only_sequence_and_envelope_identity() -> None:
    factory = ConsultationRuntimeEventFactory(conversation_id="conversation-1")
    payload = TextDeltaRuntimeEvent(delta="hello")

    first = factory.next(payload, run_id="run-1")
    second = factory.next(StreamDoneRuntimeEvent(), run_id="run-1")

    assert first.seq == 1
    assert second.seq == 2
    assert first.conversation_id == "conversation-1"
    assert first.run_id == "run-1"
    assert first.event is payload
    assert second.conversation_id == "conversation-1"


def test_runtime_event_factory_keeps_tool_identity_in_envelope() -> None:
    event = ConsultationRuntimeEventFactory(conversation_id="conversation-1").next(
        TextDeltaRuntimeEvent(delta="hello"),
        run_id="run-1",
        tool_call_id="tool-1",
    )

    assert event.tool_call_id == "tool-1"
