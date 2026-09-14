from __future__ import annotations

import json
from pathlib import Path

import pytest
from google.protobuf import json_format
from protovalidate import ValidationError, Validator

from src.generated.runtimeproto.bodysense.runtime.v1 import runtime_pb2

ROOT = Path(__file__).resolve().parents[4]
CORPUS = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/runtime.v1.json").read_text()
)
VALIDATOR = Validator()


def parse_and_validate(message, value: dict):
    json_format.ParseDict(value, message, ignore_unknown_fields=False)
    VALIDATOR.validate(message)
    return message


def test_runtime_proto_valid_corpus() -> None:
    parse_and_validate(runtime_pb2.StartTurnCommand(), CORPUS["start_turn"])
    parse_and_validate(runtime_pb2.ResumeInterruptCommand(), CORPUS["resume_interrupt"])
    assert len(CORPUS["events"]) == 17
    for value in CORPUS["events"]:
        event = parse_and_validate(runtime_pb2.RuntimeEvent(), value)
        assert event.WhichOneof("event") is not None


@pytest.mark.parametrize("case", CORPUS["invalid"], ids=lambda case: case["name"])
def test_runtime_proto_invalid_corpus(case: dict) -> None:
    message_type = case["message"]
    if message_type == "start_turn":
        message = runtime_pb2.StartTurnCommand()
    elif message_type == "resume_interrupt":
        message = runtime_pb2.ResumeInterruptCommand()
    elif message_type == "runtime_event":
        message = runtime_pb2.RuntimeEvent()
    else:
        raise AssertionError(f"unknown fixture message type {message_type!r}")

    with pytest.raises((json_format.ParseError, ValidationError)):
        parse_and_validate(message, case["value"])
