"""Generated Proto boundary adapters for the private Go↔Python runtime seam.

Generated protobuf messages stop here. The runtime route converts validated
messages back into ordinary Python dictionaries/Pydantic inputs before calling
LangGraph so transport types never become checkpoint state.
"""

from __future__ import annotations

import json
from typing import Any, TypeVar

from google.protobuf import json_format
from google.protobuf import message as protobuf_message
from protovalidate import ValidationError, Validator

from ..generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2
from ..models.stream_event import StreamEvent

_RUNTIME_VALIDATOR = Validator()
_MessageT = TypeVar("_MessageT", bound=protobuf_message.Message)


class RuntimeCommandError(ValueError):
    """The private runtime command is malformed or conflicts with its HTTP path."""



class RuntimeEventError(ValueError):
    """An internal runtime event cannot be represented by the canonical Proto IDL."""


_RUNTIME_EVENT_FIELD_BY_TYPE: dict[str, str] = {
    "runtime.agent_configuration": "agent_configuration",
    "message.text.delta": "text_delta",
    "tool.call": "tool_call",
    "tool.result": "tool_result",
    "state.extracted_info.upsert": "extracted_info",
    "state.lifestyle_context.upsert": "lifestyle_context",
    "state.interaction.required": "interaction_required",
    "state.phase.changed": "phase_changed",
    "source.citation.added": "citation_added",
    "source.answer_attribution.added": "answer_attribution_added",
    "source.knowledge_gap": "knowledge_gap",
    "safety.red_flag.detected": "red_flag_detected",
    "safety.output_reviewed": "output_reviewed",
    "safety.output_rejected": "output_rejected",
    "usage.reported": "usage_reported",
    "stream.done": "stream_done",
    "stream.error": "stream_error",
}


def runtime_event_to_proto(event: StreamEvent) -> runtime_pb2.RuntimeEvent:
    """Project a handwritten Python runtime event into the canonical private Proto.

    The generic StreamEvent remains an in-process Python runtime model for now; it
    never crosses the Go boundary after this adapter. Unknown event types fail
    closed rather than falling through to a generic payload.
    """

    field_name = _RUNTIME_EVENT_FIELD_BY_TYPE.get(event.type)
    if field_name is None:
        raise RuntimeEventError(f"unsupported private runtime event type: {event.type}")

    ids: dict[str, Any] = {
        "conversation_id": event.ids.conversation_id,
        "run_id": event.ids.run_id,
    }
    if event.ids.tool_call_id:
        ids["tool_call_id"] = event.ids.tool_call_id
    if event.ids.interaction_id:
        ids["interaction_id"] = event.ids.interaction_id

    value: dict[str, Any] = {
        "version": event.version,
        "seq": event.seq,
        "ids": ids,
        field_name: event.payload,
    }
    try:
        message = runtime_pb2.RuntimeEvent()
        json_format.ParseDict(value, message, ignore_unknown_fields=False)
        _RUNTIME_VALIDATOR.validate(message)
    except (json_format.ParseError, ValidationError, TypeError, ValueError) as exc:
        raise RuntimeEventError(f"invalid private runtime event: {event.type}") from exc
    return message


def serialize_runtime_event(event: StreamEvent) -> str:
    """Serialize one validated internal Proto event as a single NDJSON record."""

    message = runtime_event_to_proto(event)
    value = json_format.MessageToDict(
        message,
        preserving_proto_field_name=True,
        always_print_fields_with_no_presence=True,
    )
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n"


def _parse_and_validate(payload: dict[str, Any], message: _MessageT) -> _MessageT:
    try:
        json_format.ParseDict(payload, message, ignore_unknown_fields=False)
        _RUNTIME_VALIDATOR.validate(message)
    except (json_format.ParseError, ValidationError, TypeError, ValueError) as exc:
        raise RuntimeCommandError("invalid private runtime command") from exc
    return message


def _message_to_runtime_input(message: protobuf_message.Message) -> dict[str, Any]:
    value = json_format.MessageToDict(
        message,
        preserving_proto_field_name=True,
        always_print_fields_with_no_presence=False,
    )
    if not isinstance(value, dict):
        raise RuntimeCommandError("private runtime command did not decode to an object")
    return value


def parse_start_turn_command(thread_id: str, payload: dict[str, Any]) -> dict[str, Any]:
    command = _parse_and_validate(payload, runtime_pb2.StartTurnCommand())
    if command.thread_id != thread_id:
        raise RuntimeCommandError("runtime thread identity mismatch")
    value = _message_to_runtime_input(command)
    value.pop("thread_id", None)
    return value


def parse_resume_interrupt_command(
    thread_id: str,
    interrupt_id: str,
    payload: dict[str, Any],
) -> dict[str, Any]:
    command = _parse_and_validate(payload, runtime_pb2.ResumeInterruptCommand())
    if command.thread_id != thread_id:
        raise RuntimeCommandError("runtime thread identity mismatch")
    if command.interrupt_id != interrupt_id:
        raise RuntimeCommandError("runtime interrupt identity mismatch")
    value = _message_to_runtime_input(command)
    value.pop("thread_id", None)
    return value
