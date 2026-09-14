from __future__ import annotations

import pytest

from src.models.consultation_runtime_event import (
    ConsultationRuntimeEventFactory,
    StreamErrorRuntimeEvent,
)
from src.models.consultation_writer_event import (
    ConsultationWriterProtocolError,
    DoneWriterSentinel,
    StreamErrorWriterEvent,
    parse_consultation_writer_event,
)
from src.runtime.consultation_thread import _map_internal_event


def test_writer_parser_rejects_unknown_and_incomplete_values() -> None:
    with pytest.raises(ConsultationWriterProtocolError):
        parse_consultation_writer_event({"type": "future_event", "value": 1})

    with pytest.raises(ConsultationWriterProtocolError):
        parse_consultation_writer_event({"type": "tool_call", "id": "call-1", "tool": "search"})


def test_stream_error_is_not_silently_dropped() -> None:
    parsed = parse_consultation_writer_event(
        {"type": "stream_error", "message": "tool loop exceeded"}
    )
    assert isinstance(parsed, StreamErrorWriterEvent)

    event = _map_internal_event(
        ConsultationRuntimeEventFactory(conversation_id="conversation-1"),
        {"type": "stream_error", "message": "tool loop exceeded"},
        run_id="run-1",
    )

    assert event is not None
    assert isinstance(event.event, StreamErrorRuntimeEvent)
    assert event.conversation_id == "conversation-1"
    assert event.run_id == "run-1"
    assert event.event.message == "tool loop exceeded"


def test_done_writer_value_is_an_explicit_non_protocol_sentinel() -> None:
    raw = {
        "type": "__done__",
        "session_id": "conversation-1",
        "full_text": "done",
        "extracted_info": [],
        "phase": "ready_for_analysis",
    }
    parsed = parse_consultation_writer_event(raw)
    assert isinstance(parsed, DoneWriterSentinel)

    assert (
        _map_internal_event(
            ConsultationRuntimeEventFactory(conversation_id="conversation-1"),
            raw,
            run_id="run-1",
        )
        is None
    )
