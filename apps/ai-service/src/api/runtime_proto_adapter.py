"""Generated Proto boundary adapters for the private Go↔Python runtime seam.

Generated protobuf messages stop here. The runtime route converts validated
messages back into ordinary Python dictionaries/Pydantic inputs before calling
LangGraph so transport types never become checkpoint state.
"""

from __future__ import annotations

from typing import Any, TypeVar

from google.protobuf import json_format
from google.protobuf import message as protobuf_message
from protovalidate import ValidationError, Validator

from ..generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2

_RUNTIME_VALIDATOR = Validator()
_MessageT = TypeVar("_MessageT", bound=protobuf_message.Message)


class RuntimeCommandError(ValueError):
    """The private runtime command is malformed or conflicts with its HTTP path."""


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
