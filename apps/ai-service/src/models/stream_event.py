"""Structured stream event contract shared across Python, Go, and Web."""

from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field

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
    ids: StreamEventIds = Field(default_factory=StreamEventIds)
    payload: dict[str, Any] = Field(default_factory=dict)


class StreamEventFactory:
    """Build StreamEvent objects with monotonically increasing sequence numbers."""

    def __init__(self, *, conversation_id: str) -> None:
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
