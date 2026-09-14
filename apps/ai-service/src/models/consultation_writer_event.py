"""Typed values emitted by the LangGraph consultation custom writer.

This is an in-process trust boundary. LangGraph's custom stream API exposes
writer values as untyped Python objects, so every value is parsed once here
before it can become a private Go/Python runtime event.
"""

from __future__ import annotations

from typing import Annotated, Any, Literal, TypeAlias

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter, ValidationError


class ConsultationWriterProtocolError(ValueError):
    """A LangGraph custom-writer value violates the finite runtime vocabulary."""


class _WriterEvent(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)


class TextDeltaWriterEvent(_WriterEvent):
    type: Literal["text_delta"]
    delta: str


class ToolCallWriterEvent(_WriterEvent):
    type: Literal["tool_call"]
    id: str = Field(min_length=1)
    tool: str = Field(min_length=1)
    args: dict[str, Any]


class ToolResultWriterEvent(_WriterEvent):
    type: Literal["tool_result"]
    id: str = Field(min_length=1)
    tool: str = Field(min_length=1)
    result: dict[str, Any]


class ExtractedInfoWriterEvent(_WriterEvent):
    type: Literal["extracted_info"]
    info: dict[str, Any]


class LifestyleContextWriterEvent(_WriterEvent):
    type: Literal["lifestyle_context"]
    context: dict[str, Any]


class PhaseChangeWriterEvent(_WriterEvent):
    type: Literal["phase_change"]
    phase: str = Field(min_length=1)
    reason: str = Field(min_length=1)


class CitationWriterEvent(_WriterEvent):
    type: Literal["citation"]
    citation: dict[str, Any]


class AnswerAttributionWriterEvent(_WriterEvent):
    type: Literal["answer_attribution"]
    attribution: dict[str, Any]


class KnowledgeGapWriterEvent(_WriterEvent):
    type: Literal["knowledge_gap"]
    query: str
    message: str = Field(min_length=1)


class RedFlagWriterEvent(_WriterEvent):
    type: Literal["red_flag"]
    has_red_flags: bool
    flags: list[dict[str, Any]]


class UsageWriterEvent(_WriterEvent):
    type: Literal["usage"]
    usage: dict[str, Any]


class StreamErrorWriterEvent(_WriterEvent):
    type: Literal["stream_error"]
    message: str = Field(min_length=1)


class DoneWriterSentinel(_WriterEvent):
    """LangGraph completion marker; it is not a runtime protocol event."""

    type: Literal["__done__"]
    session_id: str
    full_text: str
    extracted_info: list[dict[str, Any]]
    phase: str


ConsultationWriterEvent: TypeAlias = Annotated[
    TextDeltaWriterEvent
    | ToolCallWriterEvent
    | ToolResultWriterEvent
    | ExtractedInfoWriterEvent
    | LifestyleContextWriterEvent
    | PhaseChangeWriterEvent
    | CitationWriterEvent
    | AnswerAttributionWriterEvent
    | KnowledgeGapWriterEvent
    | RedFlagWriterEvent
    | UsageWriterEvent
    | StreamErrorWriterEvent
    | DoneWriterSentinel,
    Field(discriminator="type"),
]

_WRITER_EVENT_ADAPTER: TypeAdapter[ConsultationWriterEvent] = TypeAdapter(ConsultationWriterEvent)


def parse_consultation_writer_event(value: Any) -> ConsultationWriterEvent:
    """Validate one untrusted LangGraph custom-writer value exactly once."""

    try:
        return _WRITER_EVENT_ADAPTER.validate_python(value, strict=True)
    except ValidationError as exc:
        raise ConsultationWriterProtocolError("invalid consultation writer event") from exc
