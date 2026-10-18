"""PC-RET-001: the model's raw history follows Go's context cursor."""

from __future__ import annotations

import copy
import json
from pathlib import Path
from typing import Any

import pytest

from src.api.routes.runtime import StartTurnRequest
from src.api.runtime_proto_adapter import RuntimeCommandError, parse_start_turn_command
from src.runtime.consultation_thread import (
    MAX_CONTEXT_TURNS,
    ConsultationThreadState,
    _get_conversation_text,
    _runtime_messages_to_chat_messages,
    context_history,
)

RUN_1 = "11111111-1111-4111-8111-000000000001"
RUN_2 = "11111111-1111-4111-8111-000000000002"
RUN_3 = "11111111-1111-4111-8111-000000000003"

ROOT = Path(__file__).resolve().parents[4]
CORPUS = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/runtime.v1.json").read_text()
)


def _segment(run_id: str | None, text: str) -> list[dict[str, Any]]:
    user: dict[str, Any] = {"role": "user", "content": f"user {text}"}
    if run_id:
        user["run_id"] = run_id
    return [
        user,
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{"id": f"call-{text}", "name": "lookup", "arguments": {}}],
        },
        {"role": "tool", "tool_call_id": f"call-{text}", "content": f"tool {text}"},
        {"role": "assistant", "content": f"assistant {text}"},
    ]


def _state(window: dict[str, Any] | None, run_id: str = RUN_3) -> ConsultationThreadState:
    return {
        "run_id": run_id,
        "profile": {},
        "body_state": {},
        "relevant_history": [],
        "runtime_messages": [
            *_segment(None, "legacy"),
            *_segment(RUN_1, "one"),
            *_segment(RUN_2, "two"),
            *_segment(RUN_3, "three"),
        ],
        "extracted_symptoms": [],
        "context_window": window,
    }


def _texts(messages: list[dict[str, Any]]) -> list[str]:
    return [str(m.get("content")) for m in messages if m.get("role") == "user"]


def test_history_starts_at_the_first_run_of_the_go_window() -> None:
    history = context_history(_state({"context_start_seq": 5, "run_ids": [RUN_2, RUN_3]}))
    assert _texts(history) == ["user two", "user three"]
    # Whole segments: the window never splits a Run's tool pair.
    assert history[0]["run_id"] == RUN_2
    assert history[2]["role"] == "tool"


def test_current_run_is_always_in_context() -> None:
    history = context_history(_state({"context_start_seq": 9, "run_ids": []}))
    assert _texts(history) == ["user three"]


def test_untagged_legacy_entries_never_reenter_a_window() -> None:
    history = context_history(_state({"context_start_seq": 1, "run_ids": [RUN_1]}))
    assert _texts(history) == ["user one", "user two", "user three"]


def test_window_without_any_matching_run_sends_no_history() -> None:
    state = _state({"context_start_seq": 9, "run_ids": []}, run_id="")
    state["runtime_messages"] = state["runtime_messages"][:8]
    assert context_history(state) == []


def test_missing_window_falls_back_to_the_bounded_tail() -> None:
    state = _state(None)
    state["runtime_messages"] = [
        {"role": "user", "content": f"m{i}", "run_id": RUN_1} for i in range(100)
    ]
    history = context_history(state)
    assert len(history) == MAX_CONTEXT_TURNS * 4
    assert history[-1]["content"] == "m99"


def test_window_is_not_capped_by_turn_count() -> None:
    runs = [f"11111111-1111-4111-8111-{i:012d}" for i in range(30)]
    messages: list[dict[str, Any]] = []
    for run in runs:
        messages.extend(_segment(run, run[-4:]))
    state = _state({"context_start_seq": 1, "run_ids": runs}, run_id=runs[-1])
    state["runtime_messages"] = messages
    assert len(context_history(state)) == len(messages)


def test_model_messages_and_safety_scan_honor_the_window() -> None:
    state = _state({"context_start_seq": 5, "run_ids": [RUN_2]})
    model_messages = _runtime_messages_to_chat_messages(state)
    contents = [str(m.content) for m in model_messages[1:]]
    assert "user one" not in contents and "user legacy" not in contents
    assert "user two" in contents and "user three" in contents
    text = _get_conversation_text(state)
    assert "one" not in text and "legacy" not in text
    assert "user two" in text


def _start_turn_payload(window: dict[str, Any] | None) -> dict[str, Any]:
    payload = copy.deepcopy(CORPUS["start_turn"])
    if window is not None:
        payload["business_context"]["context_window"] = window
    return payload


def test_start_turn_command_carries_the_window_to_the_runtime() -> None:
    payload = _start_turn_payload({"context_start_seq": 7, "run_ids": [RUN_1, RUN_2]})
    request = StartTurnRequest.model_validate(
        parse_start_turn_command(payload["thread_id"], payload)
    )
    window = request.business_context.context_window
    assert window is not None
    assert window.context_start_seq == 7
    assert window.run_ids == [RUN_1, RUN_2]


def test_start_turn_command_without_window_keeps_the_fallback() -> None:
    payload = _start_turn_payload(None)
    request = StartTurnRequest.model_validate(
        parse_start_turn_command(payload["thread_id"], payload)
    )
    assert request.business_context.context_window is None


@pytest.mark.parametrize(
    "window",
    [
        {"context_start_seq": 0, "run_ids": []},
        {"context_start_seq": 1, "run_ids": ["run-1"]},
        {"context_start_seq": 1, "run_ids": [RUN_1, RUN_1]},
    ],
)
def test_start_turn_command_rejects_an_invalid_window(window: dict[str, Any]) -> None:
    payload = _start_turn_payload(window)
    with pytest.raises(RuntimeCommandError):
        parse_start_turn_command(payload["thread_id"], payload)
