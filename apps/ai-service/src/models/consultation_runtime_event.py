"""Typed in-process events for the private Consultation runtime seam.

These handwritten values live between the LangGraph runtime and the generated
Proto boundary adapter. They are application types, not transport types: there
is no public StreamEvent channel and no generic payload dictionary envelope.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, TypeAlias


@dataclass(frozen=True, slots=True)
class AgentConfigurationRuntimeEvent:
    agent_configuration: dict[str, Any]
    execution_provenance: dict[str, Any]


@dataclass(frozen=True, slots=True)
class TextDeltaRuntimeEvent:
    delta: str


@dataclass(frozen=True, slots=True)
class ToolCallRuntimeEvent:
    tool: str
    args: dict[str, Any]


@dataclass(frozen=True, slots=True)
class ToolResultRuntimeEvent:
    tool: str
    result: dict[str, Any]


@dataclass(frozen=True, slots=True)
class ExtractedInfoRuntimeEvent:
    info: dict[str, Any]


@dataclass(frozen=True, slots=True)
class LifestyleContextRuntimeEvent:
    context: dict[str, Any]


@dataclass(frozen=True, slots=True)
class InteractionRequiredRuntimeEvent:
    interaction_id: str
    question: dict[str, Any]


@dataclass(frozen=True, slots=True)
class PhaseChangedRuntimeEvent:
    to: str
    reason: str
    from_phase: str | None = None


@dataclass(frozen=True, slots=True)
class CitationAddedRuntimeEvent:
    citation: dict[str, Any]


@dataclass(frozen=True, slots=True)
class AnswerAttributionAddedRuntimeEvent:
    attribution: dict[str, Any]


@dataclass(frozen=True, slots=True)
class KnowledgeGapRuntimeEvent:
    query: str
    message: str


@dataclass(frozen=True, slots=True)
class RedFlagDetectedRuntimeEvent:
    has_red_flags: bool
    flags: list[dict[str, Any]]


@dataclass(frozen=True, slots=True)
class SafetyOutputReviewedRuntimeEvent:
    kind: str
    verdict: str
    reasons: list[str]
    issues: list[dict[str, Any]]
    safety_fallback: str | None = None


@dataclass(frozen=True, slots=True)
class SafetyOutputRejectedRuntimeEvent:
    kind: str
    verdict: str
    reasons: list[str]
    issues: list[dict[str, Any]]
    safety_fallback: str | None = None


@dataclass(frozen=True, slots=True)
class UsageReportedRuntimeEvent:
    usage: dict[str, Any]


@dataclass(frozen=True, slots=True)
class StreamDoneRuntimeEvent:
    response_id: str | None = None
    usage: dict[str, Any] | None = None
    governance: dict[str, Any] | None = None


@dataclass(frozen=True, slots=True)
class StreamErrorRuntimeEvent:
    message: str


ConsultationRuntimeEventPayload: TypeAlias = (
    AgentConfigurationRuntimeEvent
    | TextDeltaRuntimeEvent
    | ToolCallRuntimeEvent
    | ToolResultRuntimeEvent
    | ExtractedInfoRuntimeEvent
    | LifestyleContextRuntimeEvent
    | InteractionRequiredRuntimeEvent
    | PhaseChangedRuntimeEvent
    | CitationAddedRuntimeEvent
    | AnswerAttributionAddedRuntimeEvent
    | KnowledgeGapRuntimeEvent
    | RedFlagDetectedRuntimeEvent
    | SafetyOutputReviewedRuntimeEvent
    | SafetyOutputRejectedRuntimeEvent
    | UsageReportedRuntimeEvent
    | StreamDoneRuntimeEvent
    | StreamErrorRuntimeEvent
)


@dataclass(frozen=True, slots=True)
class ConsultationRuntimeEvent:
    seq: int
    conversation_id: str
    run_id: str
    event: ConsultationRuntimeEventPayload
    tool_call_id: str | None = None


class ConsultationRuntimeEventFactory:
    """Assign monotonic sequence and stable stream identities to typed payloads."""

    def __init__(self, *, conversation_id: str) -> None:
        self._seq = 0
        self._conversation_id = conversation_id

    def next(
        self,
        event: ConsultationRuntimeEventPayload,
        *,
        run_id: str,
        tool_call_id: str | None = None,
    ) -> ConsultationRuntimeEvent:
        self._seq += 1
        return ConsultationRuntimeEvent(
            seq=self._seq,
            conversation_id=self._conversation_id,
            run_id=run_id,
            event=event,
            tool_call_id=tool_call_id,
        )
