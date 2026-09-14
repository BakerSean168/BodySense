from __future__ import annotations

import json
from pathlib import Path

import pytest

from src.api.runtime_proto_adapter import (
    RuntimeCommandError,
    RuntimeEventError,
    parse_resume_interrupt_command,
    parse_start_turn_command,
    runtime_event_to_proto,
    serialize_runtime_event,
)
from src.models.stream_event import StreamEvent, StreamEventIds

ROOT = Path(__file__).resolve().parents[4]
CORPUS = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/runtime.v1.json").read_text()
)


def test_start_turn_proto_boundary_projects_to_runtime_input() -> None:
    command = CORPUS["start_turn"]
    runtime_input = parse_start_turn_command(command["thread_id"], command)
    assert "thread_id" not in runtime_input
    assert runtime_input["run_id"] == command["run_id"]
    assert runtime_input["configuration_id"] == command["configuration_id"]
    assert runtime_input["business_context"]["runtime_state"]["phase"] == "collecting"


def test_start_turn_proto_boundary_rejects_path_identity_mismatch() -> None:
    with pytest.raises(RuntimeCommandError, match="thread identity mismatch"):
        parse_start_turn_command("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", CORPUS["start_turn"])


def test_resume_proto_boundary_projects_to_runtime_input() -> None:
    command = CORPUS["resume_interrupt"]
    runtime_input = parse_resume_interrupt_command(
        command["thread_id"], command["interrupt_id"], command
    )
    assert runtime_input["interrupt_id"] == command["interrupt_id"]
    assert runtime_input["answer"] == {"value": "yes"}


def test_resume_proto_boundary_rejects_interrupt_path_mismatch() -> None:
    command = CORPUS["resume_interrupt"]
    with pytest.raises(RuntimeCommandError, match="interrupt identity mismatch"):
        parse_resume_interrupt_command(
            command["thread_id"],
            "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            command,
        )

_EVENT_PROJECTIONS = {
    "agent_configuration": ("runtime", "runtime.agent_configuration"),
    "text_delta": ("message", "message.text.delta"),
    "tool_call": ("tool", "tool.call"),
    "tool_result": ("tool", "tool.result"),
    "extracted_info": ("state", "state.extracted_info.upsert"),
    "lifestyle_context": ("state", "state.lifestyle_context.upsert"),
    "interaction_required": ("state", "state.interaction.required"),
    "phase_changed": ("state", "state.phase.changed"),
    "citation_added": ("source", "source.citation.added"),
    "answer_attribution_added": ("source", "source.answer_attribution.added"),
    "knowledge_gap": ("source", "source.knowledge_gap"),
    "red_flag_detected": ("safety", "safety.red_flag.detected"),
    "output_reviewed": ("safety", "safety.output_reviewed"),
    "output_rejected": ("safety", "safety.output_rejected"),
    "usage_reported": ("usage", "usage.reported"),
    "stream_done": ("stream", "stream.done"),
    "stream_error": ("stream", "stream.error"),
}


def _legacy_event_from_proto_fixture(value: dict) -> tuple[str, StreamEvent]:
    variant = next(key for key in _EVENT_PROJECTIONS if key in value)
    channel, event_type = _EVENT_PROJECTIONS[variant]
    ids = value["ids"]
    event = StreamEvent(
        version=value["version"],
        seq=int(value["seq"]),
        channel=channel,
        type=event_type,
        ids=StreamEventIds(**ids),
        payload=value[variant],
    )
    return variant, event


def test_all_python_runtime_event_variants_project_to_proto_oneof() -> None:
    assert len(CORPUS["events"]) == len(_EVENT_PROJECTIONS) == 17
    for fixture in CORPUS["events"]:
        expected_variant, event = _legacy_event_from_proto_fixture(fixture)
        proto = runtime_event_to_proto(event)
        assert proto.WhichOneof("event") == expected_variant

        wire = json.loads(serialize_runtime_event(event))
        assert expected_variant in wire
        assert wire[expected_variant] == fixture[expected_variant]
        assert "channel" not in wire
        assert "type" not in wire
        assert "payload" not in wire


def test_python_runtime_event_boundary_rejects_unknown_event_type() -> None:
    event = StreamEvent(
        seq=1,
        channel="stream",
        type="stream.unknown",
        ids=StreamEventIds(
            conversation_id="33333333-3333-4333-8333-333333333333",
            run_id="22222222-2222-4222-8222-222222222222",
        ),
        payload={},
    )
    with pytest.raises(RuntimeEventError, match="unsupported private runtime event type"):
        runtime_event_to_proto(event)


def test_python_runtime_event_boundary_rejects_missing_run_identity() -> None:
    event = StreamEvent(
        seq=1,
        channel="message",
        type="message.text.delta",
        ids=StreamEventIds(conversation_id="33333333-3333-4333-8333-333333333333"),
        payload={"delta": "hello"},
    )
    with pytest.raises(RuntimeEventError, match="invalid private runtime event"):
        runtime_event_to_proto(event)
