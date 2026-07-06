import json
from pathlib import Path

from src.models.stream_event import StreamEvent, StreamEventFactory, StreamEventIds


def test_stream_event_factory_builds_versioned_envelope():
    factory = StreamEventFactory(conversation_id="conv-1")

    event = factory.next(
        channel="message",
        event_type="message.text.delta",
        payload={"delta": "hello"},
        ids=StreamEventIds(message_id="msg-1"),
    )

    assert event.version == 1
    assert event.seq == 1
    assert event.channel == "message"
    assert event.type == "message.text.delta"
    assert event.ids.conversation_id == "conv-1"
    assert event.ids.message_id == "msg-1"
    assert event.payload == {"delta": "hello"}


def test_stream_event_factory_increments_seq():
    factory = StreamEventFactory(conversation_id="conv-1")

    first = factory.next(channel="stream", event_type="stream.done")
    second = factory.next(channel="stream", event_type="stream.done")

    assert first.seq == 1
    assert second.seq == 2


def test_stream_event_fixture_parity():
    fixture_path = (
        Path(__file__).parents[4]
        / "packages"
        / "contracts"
        / "fixtures"
        / "stream-events.v1.json"
    )
    data = json.loads(fixture_path.read_text(encoding="utf-8"))
    parsed = [StreamEvent.model_validate(item) for item in data]

    assert parsed[1].channel == "job"
    assert parsed[1].ids.job_id == "job-1"
    assert parsed[2].ids.interaction_id == "interaction-1"
