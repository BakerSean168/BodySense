"""Generated Proto boundary adapters for the private Go↔Python runtime seam.

Generated protobuf messages stop here. The runtime route converts validated
messages back into ordinary Python dictionaries/Pydantic inputs before calling
LangGraph so transport types never become checkpoint state.
"""

from __future__ import annotations

import json
from typing import Any, TypeVar, assert_never

from google.protobuf import json_format
from google.protobuf import message as protobuf_message
from protovalidate import ValidationError, Validator

from ..generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2
from ..models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    AnswerAttributionAddedRuntimeEvent,
    CitationAddedRuntimeEvent,
    ConsultationRuntimeEvent,
    ExtractedInfoRuntimeEvent,
    InteractionRequiredRuntimeEvent,
    KnowledgeGapRuntimeEvent,
    LifestyleContextRuntimeEvent,
    PhaseChangedRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    SafetyOutputRejectedRuntimeEvent,
    SafetyOutputReviewedRuntimeEvent,
    StreamDoneRuntimeEvent,
    StreamErrorRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
    UsageReportedRuntimeEvent,
)

_RUNTIME_VALIDATOR = Validator()
_MessageT = TypeVar("_MessageT", bound=protobuf_message.Message)


class RuntimeCommandError(ValueError):
    """The private runtime command is malformed or conflicts with its HTTP path."""



class RuntimeEventError(ValueError):
    """An internal runtime event cannot be represented by the canonical Proto IDL."""


def _parse_runtime_payload(value: dict[str, Any], message: _MessageT) -> _MessageT:
    try:
        json_format.ParseDict(value, message, ignore_unknown_fields=False)
    except (json_format.ParseError, TypeError, ValueError) as exc:
        raise RuntimeEventError("invalid typed private runtime payload") from exc
    return message


def runtime_event_to_proto(event: ConsultationRuntimeEvent) -> runtime_pb2.RuntimeEvent:
    """Project one trusted handwritten runtime variant into canonical Proto."""

    ids = runtime_pb2.RuntimeEventIds(
        conversation_id=event.conversation_id,
        run_id=event.run_id,
    )
    if event.tool_call_id:
        ids.tool_call_id = event.tool_call_id

    wire = runtime_pb2.RuntimeEvent(version=1, seq=event.seq, ids=ids)
    payload = event.event

    if isinstance(payload, AgentConfigurationRuntimeEvent):
        wire.agent_configuration.CopyFrom(
            _parse_runtime_payload(
                {
                    "agent_configuration": payload.agent_configuration,
                    "execution_provenance": payload.execution_provenance,
                },
                runtime_pb2.AgentConfigurationHandshake(),
            )
        )
    elif isinstance(payload, TextDeltaRuntimeEvent):
        wire.text_delta.CopyFrom(runtime_pb2.MessageTextDelta(delta=payload.delta))
    elif isinstance(payload, ToolCallRuntimeEvent):
        wire.tool_call.CopyFrom(
            _parse_runtime_payload(
                {"tool": payload.tool, "args": payload.args},
                runtime_pb2.ToolCall(),
            )
        )
    elif isinstance(payload, ToolResultRuntimeEvent):
        wire.tool_result.CopyFrom(
            _parse_runtime_payload(
                {"tool": payload.tool, "result": payload.result},
                runtime_pb2.ToolResult(),
            )
        )
    elif isinstance(payload, ExtractedInfoRuntimeEvent):
        wire.extracted_info.CopyFrom(
            _parse_runtime_payload({"info": payload.info}, runtime_pb2.ExtractedInfoUpsert())
        )
    elif isinstance(payload, LifestyleContextRuntimeEvent):
        wire.lifestyle_context.CopyFrom(
            _parse_runtime_payload(
                {"context": payload.context},
                runtime_pb2.LifestyleContextUpsert(),
            )
        )
    elif isinstance(payload, InteractionRequiredRuntimeEvent):
        wire.ids.interaction_id = payload.interaction_id
        wire.interaction_required.CopyFrom(
            _parse_runtime_payload(
                {"interaction_id": payload.interaction_id, "question": payload.question},
                runtime_pb2.InteractionRequired(),
            )
        )
    elif isinstance(payload, PhaseChangedRuntimeEvent):
        phase_value: dict[str, Any] = {"to": payload.to, "reason": payload.reason}
        if payload.from_phase is not None:
            phase_value["from"] = payload.from_phase
        wire.phase_changed.CopyFrom(
            _parse_runtime_payload(phase_value, runtime_pb2.PhaseChanged())
        )
    elif isinstance(payload, CitationAddedRuntimeEvent):
        wire.citation_added.CopyFrom(
            _parse_runtime_payload(
                {"citation": payload.citation},
                runtime_pb2.CitationAdded(),
            )
        )
    elif isinstance(payload, AnswerAttributionAddedRuntimeEvent):
        wire.answer_attribution_added.CopyFrom(
            _parse_runtime_payload(
                {"attribution": payload.attribution},
                runtime_pb2.AnswerAttributionAdded(),
            )
        )
    elif isinstance(payload, KnowledgeGapRuntimeEvent):
        wire.knowledge_gap.CopyFrom(
            runtime_pb2.KnowledgeGap(query=payload.query, message=payload.message)
        )
    elif isinstance(payload, RedFlagDetectedRuntimeEvent):
        wire.red_flag_detected.CopyFrom(
            _parse_runtime_payload(
                {"has_red_flags": payload.has_red_flags, "flags": payload.flags},
                runtime_pb2.RedFlagDetected(),
            )
        )
    elif isinstance(payload, SafetyOutputReviewedRuntimeEvent):
        wire.output_reviewed.CopyFrom(
            _parse_runtime_payload(
                {
                    "kind": payload.kind,
                    "verdict": payload.verdict,
                    "reasons": payload.reasons,
                    "issues": payload.issues,
                    **(
                        {"safety_fallback": payload.safety_fallback}
                        if payload.safety_fallback is not None
                        else {}
                    ),
                },
                runtime_pb2.SafetyOutputEvent(),
            )
        )
    elif isinstance(payload, SafetyOutputRejectedRuntimeEvent):
        wire.output_rejected.CopyFrom(
            _parse_runtime_payload(
                {
                    "kind": payload.kind,
                    "verdict": payload.verdict,
                    "reasons": payload.reasons,
                    "issues": payload.issues,
                    **(
                        {"safety_fallback": payload.safety_fallback}
                        if payload.safety_fallback is not None
                        else {}
                    ),
                },
                runtime_pb2.SafetyOutputEvent(),
            )
        )
    elif isinstance(payload, UsageReportedRuntimeEvent):
        wire.usage_reported.CopyFrom(
            _parse_runtime_payload({"usage": payload.usage}, runtime_pb2.UsageReported())
        )
    elif isinstance(payload, StreamDoneRuntimeEvent):
        done_value: dict[str, Any] = {}
        if payload.response_id is not None:
            done_value["response_id"] = payload.response_id
        if payload.usage is not None:
            done_value["usage"] = payload.usage
        if payload.governance is not None:
            done_value["governance"] = payload.governance
        wire.stream_done.CopyFrom(_parse_runtime_payload(done_value, runtime_pb2.StreamDone()))
    elif isinstance(payload, StreamErrorRuntimeEvent):
        wire.stream_error.CopyFrom(runtime_pb2.StreamError(message=payload.message))
    else:
        assert_never(payload)

    try:
        _RUNTIME_VALIDATOR.validate(wire)
    except (ValidationError, TypeError, ValueError) as exc:
        raise RuntimeEventError(
            f"invalid private runtime event: {type(payload).__name__}"
        ) from exc
    return wire


def serialize_runtime_event(event: ConsultationRuntimeEvent) -> str:
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
