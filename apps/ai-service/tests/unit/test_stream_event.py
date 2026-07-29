import json
from pathlib import Path

from src.models.stream_event import StreamEvent, StreamEventFactory, StreamEventIds

CONTRACTS = Path(__file__).parents[4] / "packages" / "contracts"
FIXTURE_PATH = CONTRACTS / "fixtures" / "stream-events.v1.json"
SCHEMA_PATH = CONTRACTS / "schemas" / "stream-event.v1.schema.json"
REQUIRED_TYPES_PATH = CONTRACTS / "fixtures" / "stream-event-types.v1.json"

# Keep in sync with packages/contracts/schemas/stream-event.v1.schema.json
_CHANNEL_ENUM = {
    "conversation",
    "run",
    "message",
    "tool",
    "state",
    "source",
    "safety",
    "usage",
    "job",
    "stream",
    "title",
}
_ID_KEYS = {
    "conversation_id",
    "run_id",
    "turn_id",
    "message_id",
    "tool_call_id",
    "interaction_id",
    "job_id",
}
_TOP_KEYS = {"version", "seq", "channel", "type", "ids", "payload"}


def _validate_against_schema(item: dict) -> None:
    """Lightweight draft-2020-12 check for our closed stream-event schema."""
    extra = set(item) - _TOP_KEYS
    assert not extra, f"additional top-level properties: {extra}"
    for key in ("version", "seq", "channel", "type", "ids", "payload"):
        assert key in item, f"missing required field {key}"
    assert item["version"] == 1
    assert isinstance(item["seq"], int) and item["seq"] >= 1
    assert item["channel"] in _CHANNEL_ENUM, item["channel"]
    assert isinstance(item["type"], str) and len(item["type"]) >= 1
    assert isinstance(item["ids"], dict)
    assert set(item["ids"]) <= _ID_KEYS, set(item["ids"]) - _ID_KEYS
    for key, value in item["ids"].items():
        assert value is None or isinstance(value, str)
    assert isinstance(item["payload"], dict)


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
    data = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    parsed = [StreamEvent.model_validate(item) for item in data]

    assert len(parsed) >= 5
    by_type = {e.type: e for e in parsed}

    assert by_type["job.progress"].channel == "job"
    assert by_type["job.progress"].ids.job_id == "job-1"
    assert any(
        e.type == "state.interaction.required" and e.ids.interaction_id == "interaction-1"
        for e in parsed
    )
    assert "state.interaction.expired" in by_type
    assert by_type["run.started"].channel == "run"
    assert by_type["title.generated"].channel == "title"
    assert by_type["safety.output_reviewed"].channel == "safety"
    assert by_type["safety.output_rejected"].payload["verdict"] == "rejected"


def test_fixture_matches_schema():
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    assert schema["properties"]["channel"]["enum"] == sorted(_CHANNEL_ENUM) or set(
        schema["properties"]["channel"]["enum"]
    ) == _CHANNEL_ENUM

    data = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    for item in data:
        _validate_against_schema(item)


def test_fixture_covers_required_event_types():
    required = set(
        json.loads(REQUIRED_TYPES_PATH.read_text(encoding="utf-8"))[
            "required_event_types"
        ]
    )
    data = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    present = {item["type"] for item in data}
    missing = sorted(required - present)
    assert not missing, f"fixture missing required event types: {missing}"


def test_constructed_event_matches_schema_shape():
    """T0-3 C1: factory output satisfies the closed envelope schema."""
    factory = StreamEventFactory(conversation_id="conv-1")
    event = factory.next(
        channel="state",
        event_type="state.interaction.expired",
        payload={
            "interaction_id": "interaction-1",
            "expired_at": "2026-07-26T12:00:00Z",
            "reason": "ttl_elapsed",
        },
        ids=StreamEventIds(run_id="run-1", interaction_id="interaction-1", tool_call_id="tool-1"),
    )
    raw = event.model_dump(mode="json")
    _validate_against_schema(raw)


def test_fixture_includes_interaction_expired():
    events = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
    types = {item["type"] for item in events}
    assert "state.interaction.expired" in types
    expired = next(item for item in events if item["type"] == "state.interaction.expired")
    _validate_against_schema(expired)
