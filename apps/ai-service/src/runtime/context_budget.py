"""Budgeted model-request assembly for consultation history (#494).

``context_history`` (the Go context cursor) bounds history *across* Runs, but
inside one active Run a long interview — many ``ask_user`` resumes, large tool
outputs, images — can still grow without bound. This module is the request-only
counterpart: given the entries ``context_history`` selected, it fits a *copy*
of them into an explicit token budget before they become a model request.

Boundaries it must never cross:

- Checkpoint data is durable truth. Fitting only reads ``runtime_messages``
  and returns new dicts; it never rewrites state, Go cursors or evidence.
- A tool call and its results form one exchange and are never split.
- User statements are never shortened. ``ask_user`` results are user answers
  in tool clothing, so they are never shortened either. Only untrusted
  tool-result copies (knowledge search text, posture analysis, ...) may be
  truncated, and the copy says so.
- The newest user message and everything after it is indispensable. If that
  suffix alone cannot fit, fitting fails with :class:`ContextBudgetError`
  instead of silently overflowing the model window.

Token estimates mirror Go's retention estimator (``EstimateChatTokens`` in
``apps/api/internal/model/chat_retention.go``) so both sides plan against the
same numbers: 4 framing tokens per message plus one token per three UTF-8
bytes, which over-counts both CJK and ASCII for the routed BPE tokenizers.
Images are charged a fixed per-image vision cost instead of their base64
length, which is transport encoding, not model tokens.
"""

from __future__ import annotations

import json
from typing import Any

# The 32K minimum model-window plan (PC-D9): Go reserves 16K of it for raw
# history; the system prompt (profile + Go BodyState + longitudinal context),
# tool schemas and the reply share the rest.
MODEL_CONTEXT_WINDOW_TOKENS = 32_000
RAW_HISTORY_TOKEN_BUDGET = 16_000
MIN_HISTORY_TOKEN_BUDGET = 1_024
DEFAULT_RESERVED_OUTPUT_TOKENS = 2_048

MESSAGE_OVERHEAD_TOKENS = 4
BYTES_PER_TOKEN = 3
IMAGE_BLOCK_TOKEN_ESTIMATE = 1_024

# Successive caps for untrusted tool-result copies, in estimated tokens.
# Fitting tries each cap for every oversized result before it drops any
# exchange, so breadth of context is preserved as long as possible.
TOOL_RESULT_TOKEN_CAPS = (2_000, 512, 128)
TRUNCATION_MARKER = "\n[工具结果已按上下文预算截断；完整原文仍保留在检查点中]"

# Tool results that carry the user's own words, never untrusted output.
_USER_ANSWER_TOOLS = frozenset({"ask_user"})


class ContextBudgetError(RuntimeError):
    """Indispensable context cannot fit the model window; fail, don't overflow."""

    code = "context_budget_exceeded"


def estimate_text_tokens(text: str) -> int:
    """Estimate tokens for plain text with Go's bytes-per-token divisor."""
    return (len(text.encode("utf-8")) + BYTES_PER_TOKEN - 1) // BYTES_PER_TOKEN


def estimate_content_tokens(content: Any) -> int:
    """Estimate tokens for one message's content, multimodal blocks included."""
    if isinstance(content, str):
        return estimate_text_tokens(content)
    if isinstance(content, list):
        total = 0
        for block in content:
            if not isinstance(block, dict):
                total += estimate_text_tokens(str(block))
            elif block.get("type") == "text":
                total += estimate_text_tokens(str(block.get("text") or ""))
            elif block.get("type") == "image_url":
                # Vision models charge per image, not per base64 character.
                total += IMAGE_BLOCK_TOKEN_ESTIMATE
            else:
                total += estimate_text_tokens(json.dumps(block, ensure_ascii=False))
        return total
    if content is None:
        return 0
    return estimate_text_tokens(json.dumps(content, ensure_ascii=False))


def estimate_entry_tokens(entry: dict[str, Any]) -> int:
    """Estimate one runtime entry, counting tool-call arguments as payload."""
    total = MESSAGE_OVERHEAD_TOKENS + estimate_content_tokens(entry.get("content"))
    tool_calls = entry.get("tool_calls") or []
    if tool_calls:
        total += estimate_text_tokens(json.dumps(tool_calls, ensure_ascii=False))
    return total


def history_allowance(
    *,
    system_tokens: int,
    tool_overhead_tokens: int = 0,
    reserved_output_tokens: int = DEFAULT_RESERVED_OUTPUT_TOKENS,
) -> int:
    """Tokens history may use: the 16K share, less whatever the rest needs.

    The window plan is 32K total. When the system prompt (with Go BodyState),
    the tool schemas and the reserved reply together leave less than the raw
    history share, history gets what remains — BodyState and safety context
    are indispensable and are never fitted away to make room for transcript.
    """
    remaining = (
        MODEL_CONTEXT_WINDOW_TOKENS - system_tokens - tool_overhead_tokens - reserved_output_tokens
    )
    return min(RAW_HISTORY_TOKEN_BUDGET, remaining)


def _exchange_groups(entries: list[dict[str, Any]]) -> list[list[dict[str, Any]]]:
    """Group entries into exchanges that fitting may only move as a unit.

    A tool result joins the exchange opened by the preceding entry, so an
    assistant tool call and its results can never be separated; orphan tool
    results (their call already left the window) form their own exchange.
    """
    groups: list[list[dict[str, Any]]] = []
    for entry in entries:
        if entry.get("role") == "tool" and groups:
            groups[-1].append(entry)
        else:
            groups.append([entry])
    return groups


def _tool_names_by_call_id(groups: list[list[dict[str, Any]]]) -> dict[str, str]:
    names: dict[str, str] = {}
    for group in groups:
        for entry in group:
            for call in entry.get("tool_calls") or []:
                if isinstance(call, dict) and call.get("id"):
                    names[str(call["id"])] = str(call.get("name") or "")
    return names


def _shorten_text(text: str, cap_tokens: int) -> str:
    """Truncate a tool-result copy to ``cap_tokens``, marking the cut.

    Head-only: search/tool output is relevance-ordered, and the marker tells
    the model the copy is shortened while the checkpoint keeps the original.
    The cut is made on UTF-8 bytes and decoded leniently, matching the
    estimator's byte accounting.
    """
    budget_bytes = cap_tokens * BYTES_PER_TOKEN - len(TRUNCATION_MARKER.encode("utf-8"))
    encoded = text.encode("utf-8")
    if len(encoded) <= budget_bytes:
        return text
    return encoded[: max(budget_bytes, 0)].decode("utf-8", errors="ignore") + TRUNCATION_MARKER


def _shorten_tool_results(
    groups: list[list[dict[str, Any]]], tool_names: dict[str, str], cap_tokens: int
) -> None:
    """In-place (on copies) cap of every untrusted tool result over the cap."""
    for group in groups:
        for entry in group:
            if entry.get("role") != "tool":
                continue
            if tool_names.get(str(entry.get("tool_call_id") or "")) in _USER_ANSWER_TOOLS:
                continue
            content = entry.get("content")
            if isinstance(content, str) and estimate_text_tokens(content) > cap_tokens:
                entry["content"] = _shorten_text(content, cap_tokens)


def fit_history_to_budget(
    entries: list[dict[str, Any]],
    *,
    budget_tokens: int,
) -> list[dict[str, Any]]:
    """Return a request-only copy of ``entries`` fitting ``budget_tokens``.

    Fitting order: (1) progressively shorten untrusted tool-result copies,
    (2) drop the oldest complete exchanges. The suffix starting at the newest
    user message is indispensable; if it alone exceeds the budget after
    shortening, raise :class:`ContextBudgetError`. Source entries are never
    mutated, so checkpoints keep every byte the model did not see.
    """
    if budget_tokens < MIN_HISTORY_TOKEN_BUDGET:
        raise ContextBudgetError(
            f"history budget {budget_tokens} tokens is below the minimum "
            f"{MIN_HISTORY_TOKEN_BUDGET}: prompt and tool overhead already "
            "consume the model window"
        )
    if sum(estimate_entry_tokens(entry) for entry in entries) <= budget_tokens:
        return list(entries)

    groups = [[dict(entry) for entry in group] for group in _exchange_groups(entries)]
    tool_names = _tool_names_by_call_id(groups)

    def total_tokens() -> int:
        return sum(estimate_entry_tokens(entry) for group in groups for entry in group)

    for cap in TOOL_RESULT_TOKEN_CAPS:
        _shorten_tool_results(groups, tool_names, cap)
        if total_tokens() <= budget_tokens:
            return [entry for group in groups for entry in group]

    # The indispensable suffix starts at the exchange holding the newest
    # user-authored content: a user message, or an ``ask_user`` result (the
    # user's answer in tool clothing). Everything from there on survives
    # whole. With no user-authored content at all, every exchange is
    # droppable and only the budget decides.
    def is_user_authored(entry: dict[str, Any]) -> bool:
        if entry.get("role") == "user":
            return True
        return (
            entry.get("role") == "tool"
            and tool_names.get(str(entry.get("tool_call_id") or "")) in _USER_ANSWER_TOOLS
        )

    suffix_start = len(groups)
    for index, group in enumerate(groups):
        if any(is_user_authored(entry) for entry in group):
            suffix_start = index
    while total_tokens() > budget_tokens and suffix_start > 0:
        groups.pop(0)
        suffix_start -= 1
    if total_tokens() > budget_tokens:
        raise ContextBudgetError(
            f"indispensable context needs {total_tokens()} tokens but only "
            f"{budget_tokens} remain in the model window"
        )
    return [entry for group in groups for entry in group]
