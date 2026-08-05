"""Structured stream event contract shared across Python, Go, and Web.

Learning path (Thought Forest note filenames):
- python-typing-basics.md
- python-protocols-and-structural-typing.md
- python-mutability-identity-and-copying.md
- typescript-static-types-and-runtime-validation.md

Unlike TypeScript-only declarations, Pydantic models validate data at runtime.
They protect this Python boundary, while shared schema fixtures keep the Go,
Python, and TypeScript representations aligned.
"""

from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field

# Literal describes the finite protocol vocabulary to the type checker and
# Pydantic. A value outside this list fails model validation.
StreamChannel = Literal[
    "conversation",
    "run",
    "message",
    "tool",
    "state",
    "source",
    "safety",
    "usage",
    "job",
    "stream",
    "title",
]


class StreamEventIds(BaseModel):
    """Identifiers that relate an event to conversation state."""

    conversation_id: str | None = None
    run_id: str | None = None
    turn_id: str | None = None
    message_id: str | None = None
    tool_call_id: str | None = None
    interaction_id: str | None = None
    job_id: str | None = None


class StreamEvent(BaseModel):
    """Versioned event envelope for structured streaming."""

    version: Literal[1] = 1
    seq: int = Field(..., ge=1)
    channel: StreamChannel
    type: str
    # default_factory creates a fresh object for each model instance. Reusing a
    # mutable module-level dict would let one event accidentally affect another.
    ids: StreamEventIds = Field(default_factory=StreamEventIds)
    payload: dict[str, Any] = Field(default_factory=dict)


class StreamEventFactory:
    """Build StreamEvent objects with monotonically increasing sequence numbers."""

    def __init__(self, *, conversation_id: str) -> None:
        # The factory owns sequence state for one stream. `next` mutates this
        # private counter, so a factory must not be shared across independent
        # runs unless their ordering is intentionally coupled.
        self._seq = 0
        self._conversation_id = conversation_id

    def next(
        self,
        *,
        channel: StreamChannel,
        event_type: str,
        payload: dict[str, Any] | None = None,
        ids: StreamEventIds | None = None,
    ) -> StreamEvent:
        self._seq += 1
        # `or` is safe here because BaseModel instances are truthy. The caller's
        # ids object is intentionally enriched with the conversation id.
        event_ids = ids or StreamEventIds()
        if not event_ids.conversation_id:
            event_ids.conversation_id = self._conversation_id
        return StreamEvent(
            seq=self._seq,
            channel=channel,
            type=event_type,
            ids=event_ids,
            payload=payload or {},
        )
