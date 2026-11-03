"""#494: the model request is fitted to an explicit budget inside one Run.

Go's context cursor bounds history across Runs, but a single active Run can
still accumulate unbounded context through ask_user resumes, large tool
output and images. These tests pin the request-only assembler: it never
rewrites checkpoint entries, never splits a tool exchange, never shortens
user statements, and fails explicitly when indispensable context cannot fit.
"""

from __future__ import annotations

import copy
from typing import Any
from unittest.mock import patch

import pytest

from src.runtime import consultation_thread
from src.runtime.consultation_thread import (
    ConsultationThreadState,
    _runtime_messages_to_chat_messages,
    llm_turn,
)
from src.runtime.context_budget import (
    IMAGE_BLOCK_TOKEN_ESTIMATE,
    RAW_HISTORY_TOKEN_BUDGET,
    ContextBudgetError,
    estimate_content_tokens,
    estimate_entry_tokens,
    fit_history_to_budget,
    history_allowance,
)

RUN_OLD = "11111111-1111-4111-8111-00000000000a"
RUN_NEW = "11111111-1111-4111-8111-00000000000b"


def _total(entries: list[dict[str, Any]]) -> int:
    return sum(estimate_entry_tokens(entry) for entry in entries)


def _ask_exchange(index: int, *, filler_chars: int = 0) -> list[dict[str, Any]]:
    return [
        {
            "role": "assistant",
            "content": "x" * filler_chars,
            "tool_calls": [
                {
                    "id": f"ask-{index}",
                    "name": "ask_user",
                    "arguments": {"question": f"问题{index}"},
                }
            ],
        },
        {
            "role": "tool",
            "tool_call_id": f"ask-{index}",
            "content": f'{{"answer": "回答{index}"}}',
        },
    ]


def _assert_pairs_intact(entries: list[dict[str, Any]]) -> None:
    """Every tool call keeps its results and no result is orphaned."""
    seen_calls: set[str] = set()
    for position, entry in enumerate(entries):
        for call in entry.get("tool_calls") or []:
            seen_calls.add(call["id"])
            following = entries[position + 1] if position + 1 < len(entries) else None
            assert following is not None and following.get("role") == "tool", (
                f"tool call {call['id']} lost its result"
            )
        if entry.get("role") == "tool":
            assert entry.get("tool_call_id") in seen_calls, "orphaned tool result"


def test_estimator_mirrors_go_chat_retention() -> None:
    # Go: 4 framing tokens + ceil(UTF-8 bytes / 3); CJK char = 3 bytes = 1 token.
    assert estimate_entry_tokens({"role": "user", "content": "abc"}) == 5
    assert estimate_entry_tokens({"role": "user", "content": "肩膀疼"}) == 7
    images = [{"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,AAAA"}}]
    assert estimate_content_tokens(images) == IMAGE_BLOCK_TOKEN_ESTIMATE


def test_history_allowance_keeps_the_16k_share_inside_the_32k_plan() -> None:
    assert history_allowance(system_tokens=1_000) == RAW_HISTORY_TOKEN_BUDGET
    # A large BodyState/system prompt shrinks history, never the other way.
    assert history_allowance(system_tokens=20_000, reserved_output_tokens=2_048) == 9_952
    assert history_allowance(system_tokens=40_000) < 1_024


def test_many_ask_user_resumes_in_one_run_are_fitted_by_dropping_old_exchanges() -> None:
    entries: list[dict[str, Any]] = [{"role": "user", "content": "我的腰一直疼", "run_id": "run-1"}]
    for index in range(30):
        entries.extend(_ask_exchange(index, filler_chars=3_000))
    assert _total(entries) > RAW_HISTORY_TOKEN_BUDGET
    snapshot = copy.deepcopy(entries)

    fitted = fit_history_to_budget(entries, budget_tokens=RAW_HISTORY_TOKEN_BUDGET)

    assert _total(fitted) <= RAW_HISTORY_TOKEN_BUDGET
    _assert_pairs_intact(fitted)
    # The newest user answer survived, and the checkpoint source is untouched.
    assert fitted[-1]["content"] == '{"answer": "回答29"}'
    assert entries == snapshot
    # Dropping happened at exchange granularity: no fitted tool call id is
    # missing its pair and the earliest exchanges are the ones that left.
    kept_ids = {call["id"] for entry in fitted for call in entry.get("tool_calls") or []}
    assert "ask-29" in kept_ids and "ask-0" not in kept_ids


def test_large_untrusted_tool_output_is_shortened_but_user_text_is_not() -> None:
    big_result = "知识库条目。" * 8_000  # ~32K estimated tokens
    entries = [
        {"role": "user", "content": "坐骨神经痛怎么办"},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{"id": "search-1", "name": "search_knowledge", "arguments": {}}],
        },
        {"role": "tool", "tool_call_id": "search-1", "content": big_result},
        {"role": "assistant", "content": "根据资料……"},
    ]
    snapshot = copy.deepcopy(entries)

    fitted = fit_history_to_budget(entries, budget_tokens=RAW_HISTORY_TOKEN_BUDGET)

    assert _total(fitted) <= RAW_HISTORY_TOKEN_BUDGET
    tool_copy = next(entry for entry in fitted if entry["role"] == "tool")
    assert "截断" in str(tool_copy["content"])
    assert len(str(tool_copy["content"])) < len(big_result)
    user_copy = next(entry for entry in fitted if entry["role"] == "user")
    assert user_copy["content"] == "坐骨神经痛怎么办"
    # The checkpoint keeps the full tool output; only the request copy shrank.
    assert entries == snapshot


def test_ask_user_answers_are_never_shortened() -> None:
    answer = '{"answer": "' + "疼" * 6_000 + '"}'  # ~6K tokens: fits once search shrinks
    entries = [
        {"role": "user", "content": "腿疼"},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{"id": "search-1", "name": "search_knowledge", "arguments": {}}],
        },
        {"role": "tool", "tool_call_id": "search-1", "content": "条目。" * 12_000},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [{"id": "ask-0", "name": "ask_user", "arguments": {}}],
        },
        {"role": "tool", "tool_call_id": "ask-0", "content": answer},
    ]
    fitted = fit_history_to_budget(entries, budget_tokens=RAW_HISTORY_TOKEN_BUDGET)
    ask_copy = next(entry for entry in fitted if entry.get("tool_call_id") == "ask-0")
    assert ask_copy["content"] == answer


def test_multimodal_newest_message_keeps_its_images_and_fits() -> None:
    image_blocks = [
        {"type": "text", "text": "看这张照片"},
        *(
            {"type": "image_url", "image_url": {"url": f"data:image/jpeg;base64,IMG{i}"}}
            for i in range(3)
        ),
    ]
    entries: list[dict[str, Any]] = [{"role": "user", "content": "肩膀疼", "run_id": "run-1"}]
    for index in range(20):
        entries.extend(_ask_exchange(index, filler_chars=3_000))
    entries.append({"role": "user", "content": image_blocks})
    snapshot = copy.deepcopy(entries)

    fitted = fit_history_to_budget(entries, budget_tokens=RAW_HISTORY_TOKEN_BUDGET)

    assert _total(fitted) <= RAW_HISTORY_TOKEN_BUDGET
    _assert_pairs_intact(fitted)
    assert fitted[-1]["content"] == image_blocks
    assert entries == snapshot


def test_indispensable_context_that_cannot_fit_fails_explicitly() -> None:
    entries = [{"role": "user", "content": "疼" * 60_000}]  # ~60K tokens alone
    with pytest.raises(ContextBudgetError):
        fit_history_to_budget(entries, budget_tokens=RAW_HISTORY_TOKEN_BUDGET)
    with pytest.raises(ContextBudgetError):
        fit_history_to_budget([{"role": "user", "content": "你好"}], budget_tokens=512)


def _state_with_huge_current_run() -> ConsultationThreadState:
    runtime_messages: list[dict[str, Any]] = [
        {"role": "user", "content": "旧 Run 的问题", "run_id": RUN_OLD},
        {"role": "assistant", "content": "旧 Run 的回答"},
        {"role": "user", "content": "我的膝盖疼", "run_id": RUN_NEW},
    ]
    for index in range(30):
        runtime_messages.extend(_ask_exchange(index, filler_chars=3_000))
    return {
        "run_id": RUN_NEW,
        "profile": {},
        "body_state": {},
        "relevant_history": [],
        "runtime_messages": runtime_messages,
        "extracted_symptoms": [],
        "context_window": {"context_start_seq": 3, "run_ids": [RUN_NEW]},
    }


def test_model_messages_apply_cursor_then_budget_without_touching_state() -> None:
    state = _state_with_huge_current_run()
    snapshot = copy.deepcopy(state["runtime_messages"])

    messages = _runtime_messages_to_chat_messages(state)

    contents = [str(message.content) for message in messages]
    assert not any("旧 Run" in content for content in contents)  # Go cursor first
    system_tokens = estimate_content_tokens(messages[0].content)
    allowance = history_allowance(system_tokens=system_tokens)
    history_tokens = sum(
        estimate_entry_tokens(
            {
                "role": message.role,
                "content": message.content,
                "tool_calls": [
                    {"id": call.id, "name": call.name, "arguments": call.arguments}
                    for call in message.tool_calls or []
                ],
                "tool_call_id": message.tool_call_id,
            }
        )
        for message in messages[1:]
    )
    assert 0 < history_tokens <= allowance
    assert messages[-1].content == '{"answer": "回答29"}'
    assert state["runtime_messages"] == snapshot  # checkpoint source intact


@pytest.mark.asyncio
async def test_llm_turn_reports_a_stream_error_when_context_cannot_fit() -> None:
    emitted: list[dict[str, Any]] = []
    state: dict[str, Any] = {
        "runtime_messages": [{"role": "user", "content": "疼" * 200_000}],
        "tool_rounds": 0,
    }

    class NeverCalledAI:
        async def generate_stream(self, _request):  # pragma: no cover - must not run
            raise AssertionError("model must not be called with an unfittable request")
            yield  # pragma: no cover

    with (
        patch.object(consultation_thread, "_get_ai_service", return_value=NeverCalledAI()),
        patch.object(
            consultation_thread,
            "get_consultation_registry",
            return_value=type("Registry", (), {"to_provider_tools": staticmethod(lambda: [])})(),
        ),
    ):
        result = await llm_turn(state, writer=emitted.append)  # type: ignore[arg-type]

    assert result["pending_tool_calls"] == []
    assert any(
        event.get("type") == "stream_error" and "预算" in str(event.get("message"))
        for event in emitted
    )
