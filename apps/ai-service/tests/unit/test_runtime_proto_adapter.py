from __future__ import annotations

import json
from pathlib import Path

import pytest

from src.api.runtime_proto_adapter import (
    RuntimeCommandError,
    parse_resume_interrupt_command,
    parse_start_turn_command,
)

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
