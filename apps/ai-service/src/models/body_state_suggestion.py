"""Source-cited BodyState suggestions (BS-AGENT-001); never commits or receipts.

The model may only point at the user's own words in the latest user message.
Python locates the exact rune span itself, and Go re-resolves the cited message,
copies the excerpt, verifies every label against it and commits only after the
user confirms. Nothing the model writes becomes a statement.
"""

from __future__ import annotations

import re
from typing import Literal
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field

BODY_STATE_SUGGESTION_KINDS = ("hypothesis", "observation", "episode")

_SELF = re.compile(r"^(?:我(?!们)|本人|(?i:I\b|my\b))")
_THIRD_PARTY = re.compile(
    r"我(?:的)?(?:朋友|同事|妈妈|爸爸|父亲|母亲|家人|孩子|儿子|女儿|妻子|丈夫|男友|女友)|"
    r"(?i:\bmy (?:friend|mother|father|child|wife|husband)\b)"
)
_QUESTION = re.compile(r"[？?]|吗[。.!！]?$")
_CLAUSE_END = "。！？!?\n；;.，,"
_OBSERVATION_KIND = re.compile(r"^[a-z][a-z0-9_]{1,63}$")


class BodyStateSuggestionDraft(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)
    kind: Literal["hypothesis", "observation", "episode"]
    quote: str = Field(min_length=1, max_length=2000)
    observation_kind: str = Field(default="", max_length=64)
    raw_value: str = Field(default="", max_length=120)
    raw_unit: str = Field(default="", max_length=48)
    symptom_type: str = Field(default="", max_length=120)


class BodyStateSuggestionOutput(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)
    suggestion: BodyStateSuggestionDraft | None = None


def validate_suggestion(
    output: BodyStateSuggestionOutput,
    original: str,
    episode_statements: tuple[str, ...] = (),
) -> None:
    """Reject anything that is not an exact, self-authored, unambiguous span."""
    draft = output.suggestion
    if draft is None:
        return
    quote = draft.quote
    if quote != quote.strip() or not quote:
        raise ValueError("quote must not be padded")
    start = original.find(quote)
    if start < 0 or original.find(quote, start + 1) >= 0:
        raise ValueError("quote must occur exactly once, verbatim, in the user message")
    prefix = original[:start].rstrip()
    suffix = original[start + len(quote) :]
    if prefix and prefix[-1] not in _CLAUSE_END:
        raise ValueError("quote must start at a sentence or clause boundary")
    if suffix and suffix[0] not in _CLAUSE_END:
        raise ValueError("quote must end at a sentence or clause boundary")
    if not _SELF.search(quote) or _THIRD_PARTY.search(quote) or _QUESTION.search(quote):
        raise ValueError("quote is not an unambiguous first-person statement")

    def verbatim(value: str) -> bool:
        return bool(value) and value == value.strip() and value in quote

    labels = (draft.observation_kind, draft.raw_value, draft.raw_unit, draft.symptom_type)
    if draft.kind == "hypothesis" and any(labels):
        raise ValueError("a hypothesis carries no labels")
    if draft.kind == "observation":
        if not _OBSERVATION_KIND.fullmatch(draft.observation_kind) or draft.symptom_type:
            raise ValueError("an observation needs one lowercase organizational label")
        if (draft.raw_value and not verbatim(draft.raw_value)) or (
            draft.raw_unit and not verbatim(draft.raw_unit)
        ):
            raise ValueError("raw value and unit must be copied from the quote")
        if draft.raw_unit and not draft.raw_value:
            raise ValueError("a unit needs its reported value")
    if draft.kind == "episode":
        if draft.observation_kind or draft.raw_value or draft.raw_unit:
            raise ValueError("an episode carries only its symptom words")
        if not verbatim(draft.symptom_type):
            raise ValueError("symptom words must be copied from the quote")
        if quote not in episode_statements:
            raise ValueError("an episode must quote a symptom statement captured this turn")


def body_state_revision(value: object) -> int:
    """Accept only a lossless, Go-owned integral revision (Struct numbers are floats)."""
    if (
        isinstance(value, bool)
        or not isinstance(value, (int, float))
        or value < 0
        or value > 2**53 - 1
        or not float(value).is_integer()
    ):
        raise ValueError("a Go-owned lossless integer revision is required")
    return int(value)


def body_state_suggestion_envelope(
    output: BodyStateSuggestionOutput,
    original: str,
    *,
    message_id: str,
    base_revision: int,
    episode_statements: tuple[str, ...] = (),
) -> dict[str, object] | None:
    """Map a validated draft onto the typed runtime suggestion Go binds."""
    validate_suggestion(output, original, episode_statements)
    draft = output.suggestion
    if draft is None:
        return None
    cited = str(UUID(message_id))
    if cited != message_id:
        raise ValueError("cited message id must be Go's canonical id")
    start = original.index(draft.quote)
    payload: dict[str, str] = {}
    if draft.kind == "observation":
        payload["observation_kind"] = draft.observation_kind
        if draft.raw_value:
            payload["raw_value"] = draft.raw_value
        if draft.raw_unit:
            payload["raw_unit"] = draft.raw_unit
    if draft.kind == "episode":
        payload["symptom_type"] = draft.symptom_type
    envelope: dict[str, object] = {
        "kind": draft.kind,
        "cited_message_id": cited,
        # Python str indices are code points, the same rune offsets Go uses.
        "span": {"start": start, "end": start + len(draft.quote)},
        "base_body_state_revision": base_revision,
    }
    if payload:
        envelope["payload"] = payload
    return envelope
