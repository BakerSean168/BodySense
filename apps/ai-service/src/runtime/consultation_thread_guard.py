"""Hold back and inspect the final assistant tail before it reaches the user."""

from __future__ import annotations

import re

STREAM_TAIL_HOLD_CHARS = 320
_QUESTION_SENTENCE_RE = re.compile(r"(?P<question>[^。！？!?\n]{2,220}[？?])")
_RHETORICAL_HEADING_RE = re.compile(r"^\d+[.、]\s*\*\*[^*]+\*\*[？?]$")
_NON_INTERACTIVE_QUESTION_MARKERS = ("例如", "比如")
_OPTIONAL_OFFER_PREFIXES = (
    "需要我",
    "要不要我",
    "是否需要我",
    "你还需要我",
    "如果你愿意，我可以",
    "如果需要，我可以",
    "或者你有其他问题",
    "还有其他问题",
    "你还有其他问题",
)


def _is_non_interactive_question(question: str) -> bool:
    compact = question.strip()
    if any(marker in compact for marker in _NON_INTERACTIVE_QUESTION_MARKERS):
        return True
    if compact.startswith("#"):
        return True
    return bool(_RHETORICAL_HEADING_RE.fullmatch(compact))


def _guard_final_assistant_text(text: str) -> tuple[str, str | None]:
    """Keep manual-input questions out of assistant prose.

    Only the held streaming tail can be rewritten. Optional offer questions are
    dropped entirely; a real information request is returned so the runtime can
    synthesize an ``ask_user`` interaction instead. This also catches the common
    model pattern ``需要进一步确认的信息： ...？ ...`` even when the final byte is a
    colon rather than a question mark.
    """
    stripped = text.rstrip()
    tail_start = max(0, len(stripped) - STREAM_TAIL_HOLD_CHARS)
    held_tail = stripped[tail_start:]
    matches = list(_QUESTION_SENTENCE_RE.finditer(held_tail))
    interactive_matches = [
        match for match in matches if not _is_non_interactive_question(match.group("question"))
    ]
    if not interactive_matches:
        return stripped, None

    real_match: re.Match[str] | None = None
    optional_match: re.Match[str] | None = None
    for match in interactive_matches:
        compact = match.group("question").lstrip("-*# ").strip()
        if compact.startswith(_OPTIONAL_OFFER_PREFIXES):
            optional_match = optional_match or match
            continue
        real_match = match
        break

    chosen = real_match or optional_match
    if chosen is None:
        return stripped, None

    # Once the model starts a manual-question tail, none of those interactive
    # questions belong in prose. Example questions ("例如/比如") are explanatory
    # text and stay untouched because they do not ask the user to respond.
    cut_in_tail = interactive_matches[0].start()
    prefix = held_tail[:cut_in_tail]
    for marker in (
        "需要进一步确认的信息",
        "还需要确认的信息",
        "请补充以下信息",
        "为了更精准",
        "为了更准确",
    ):
        marker_index = prefix.rfind(marker)
        if marker_index >= 0:
            cut_in_tail = prefix.rfind("\n", 0, marker_index) + 1
            break

    body = stripped[: tail_start + cut_in_tail].rstrip()
    if real_match is None:
        return body, None
    question = real_match.group("question").lstrip("-*# ").strip()
    return body, question


def _chunk_text(text: str, chunk_size: int = 120) -> list[str]:
    return [text[i : i + chunk_size] for i in range(0, len(text), chunk_size)]
