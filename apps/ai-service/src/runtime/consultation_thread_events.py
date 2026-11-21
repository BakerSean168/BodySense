"""Map internal writer events to the protocol runtime event stream."""

from __future__ import annotations

from typing import Any, assert_never

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    AnswerAttributionAddedRuntimeEvent,
    CitationAddedRuntimeEvent,
    ConsultationRuntimeEvent,
    ConsultationRuntimeEventFactory,
    ExtractedInfoRuntimeEvent,
    KnowledgeGapRuntimeEvent,
    LifestyleContextRuntimeEvent,
    PhaseChangedRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamErrorRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
    UsageReportedRuntimeEvent,
)
from ..models.consultation_writer_event import (
    AnswerAttributionWriterEvent,
    CitationWriterEvent,
    DoneWriterSentinel,
    ExtractedInfoWriterEvent,
    KnowledgeGapWriterEvent,
    LifestyleContextWriterEvent,
    PhaseChangeWriterEvent,
    RedFlagWriterEvent,
    StreamErrorWriterEvent,
    TextDeltaWriterEvent,
    ToolCallWriterEvent,
    ToolResultWriterEvent,
    UsageWriterEvent,
    parse_consultation_writer_event,
)


def _map_internal_event(
    factory: ConsultationRuntimeEventFactory,
    event_data: Any,
    *,
    run_id: str,
) -> ConsultationRuntimeEvent | None:
    event = parse_consultation_writer_event(event_data)

    if isinstance(event, TextDeltaWriterEvent):
        return factory.next(TextDeltaRuntimeEvent(delta=event.delta), run_id=run_id)
    if isinstance(event, ToolCallWriterEvent):
        return factory.next(
            ToolCallRuntimeEvent(tool=event.tool, args=event.args),
            run_id=run_id,
            tool_call_id=event.id,
        )
    if isinstance(event, ToolResultWriterEvent):
        return factory.next(
            ToolResultRuntimeEvent(tool=event.tool, result=event.result),
            run_id=run_id,
            tool_call_id=event.id,
        )
    if isinstance(event, ExtractedInfoWriterEvent):
        return factory.next(ExtractedInfoRuntimeEvent(info=event.info), run_id=run_id)
    if isinstance(event, LifestyleContextWriterEvent):
        return factory.next(LifestyleContextRuntimeEvent(context=event.context), run_id=run_id)
    if isinstance(event, PhaseChangeWriterEvent):
        return factory.next(
            PhaseChangedRuntimeEvent(to=event.phase, reason=event.reason),
            run_id=run_id,
        )
    if isinstance(event, CitationWriterEvent):
        return factory.next(CitationAddedRuntimeEvent(citation=event.citation), run_id=run_id)
    if isinstance(event, AnswerAttributionWriterEvent):
        return factory.next(
            AnswerAttributionAddedRuntimeEvent(attribution=event.attribution),
            run_id=run_id,
        )
    if isinstance(event, KnowledgeGapWriterEvent):
        return factory.next(
            KnowledgeGapRuntimeEvent(query=event.query, message=event.message),
            run_id=run_id,
        )
    if isinstance(event, RedFlagWriterEvent):
        return factory.next(
            RedFlagDetectedRuntimeEvent(
                has_red_flags=event.has_red_flags,
                flags=event.flags,
            ),
            run_id=run_id,
        )
    if isinstance(event, UsageWriterEvent):
        return factory.next(UsageReportedRuntimeEvent(usage=event.usage), run_id=run_id)
    if isinstance(event, StreamErrorWriterEvent):
        return factory.next(StreamErrorRuntimeEvent(message=event.message), run_id=run_id)
    if isinstance(event, DoneWriterSentinel):
        return None
    assert_never(event)


def _runtime_agent_configuration_event(
    factory: ConsultationRuntimeEventFactory,
    *,
    manifest: ConsultationAgentManifest,
    run_id: str,
    usage: dict[str, Any] | None = None,
) -> ConsultationRuntimeEvent:
    """Build the internal control-plane identity handshake for Go."""
    return factory.next(
        AgentConfigurationRuntimeEvent(
            agent_configuration=manifest.provenance(),
            execution_provenance={
                "status": "executed",
                "runtime": "langgraph",
                "logical_model": manifest.logical_model,
                "model_group_revision": manifest.model_group_revision,
                "usage": usage or {},
            },
        ),
        run_id=run_id,
    )
