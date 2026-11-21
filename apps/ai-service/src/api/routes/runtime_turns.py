"""Streaming turn and interrupt-resume routes for consultation threads.

The nested async generators bridge domain events to an HTTP byte stream. They
yield one JSON record at a time instead of materializing the whole reply.
"""

from __future__ import annotations

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse

from ...models.consultation_runtime_event import (
    ConsultationRuntimeEvent,
    StreamErrorRuntimeEvent,
)
from ...runtime.consultation_thread import resume_thread_interrupt, stream_thread_turn
from ..runtime_proto_adapter import (
    RuntimeCommandError,
    parse_resume_interrupt_command,
    parse_start_turn_command,
    serialize_runtime_event,
)
from .runtime_e2e import _e2e_stub_enabled, e2e_resume_stream, e2e_turn_stream
from .runtime_schemas import BusinessContext, ResumeInterruptRequest, StartTurnRequest

logger = logging.getLogger(__name__)

# The ``/runtime`` prefix comes from the parent router in ``runtime``, which
# includes this one; keep it off here so the paths are applied once.
router = APIRouter(tags=["runtime"])


def _context_window(business_context: BusinessContext) -> dict[str, Any] | None:
    window = business_context.context_window
    return window.model_dump() if window is not None else None


@router.post("/threads/{thread_id}/turns")
async def start_turn(thread_id: str, payload: dict[str, Any]):
    try:
        request = StartTurnRequest.model_validate(parse_start_turn_command(thread_id, payload))
    except RuntimeCommandError as exc:
        logger.warning("Rejected invalid start-turn runtime command: %s", exc)
        raise HTTPException(status_code=422, detail="invalid private runtime command") from exc
    if _e2e_stub_enabled():
        return StreamingResponse(e2e_turn_stream(request), media_type="application/x-ndjson")

    # An async generator can await upstream work and yield records repeatedly.
    # StreamingResponse consumes it lazily and applies backpressure through the
    # ASGI server rather than building one large response in memory.
    async def ndjson_generator():
        try:
            async for event in stream_thread_turn(
                thread_id=thread_id,
                conversation_id=request.conversation_id,
                run_id=request.run_id,
                user_id=request.user_id,
                user_message=request.input.text,
                user_message_id=request.input.message_id,
                images=[img.model_dump() for img in request.input.images],
                profile=request.business_context.profile,
                body_state=request.business_context.body_state,
                extracted_info=request.business_context.runtime_state.extracted_info,
                phase=request.business_context.runtime_state.phase,
                posture_analysis=request.business_context.posture_analysis,
                relevant_history=request.business_context.relevant_history,
                episodic_notes=request.business_context.episodic_notes,
                current_diagnosis=request.business_context.current_diagnosis,
                current_treatment=request.business_context.current_treatment,
                recent_outcomes=request.business_context.recent_outcomes,
                spatial_context=(
                    request.business_context.spatial_context.model_dump(exclude_none=True)
                    if request.business_context.spatial_context
                    else None
                ),
                configuration_id=request.configuration_id,
                context_window=_context_window(request.business_context),
            ):
                yield serialize_runtime_event(event)
        except Exception:
            # The HTTP headers may already be sent, so raising an HTTPException
            # cannot reliably replace the response. Emit a protocol-level error
            # record and log the original exception server-side instead.
            logger.exception("Error in runtime thread turn")
            yield serialize_runtime_event(
                ConsultationRuntimeEvent(
                    seq=1,
                    conversation_id=request.conversation_id,
                    run_id=request.run_id,
                    event=StreamErrorRuntimeEvent(message="Internal runtime error."),
                )
            )

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        # Both headers reduce intermediary buffering so clients observe records
        # close to the time they are yielded.
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@router.post("/threads/{thread_id}/interrupts/{interrupt_id}/resume")
async def resume_interrupt(thread_id: str, interrupt_id: str, payload: dict[str, Any]):
    try:
        request = ResumeInterruptRequest.model_validate(
            parse_resume_interrupt_command(thread_id, interrupt_id, payload)
        )
    except RuntimeCommandError as exc:
        logger.warning("Rejected invalid resume runtime command: %s", exc)
        raise HTTPException(status_code=422, detail="invalid private runtime command") from exc

    if _e2e_stub_enabled():
        return StreamingResponse(e2e_resume_stream(request), media_type="application/x-ndjson")

    async def ndjson_generator():
        try:
            async for event in resume_thread_interrupt(
                thread_id=thread_id,
                conversation_id=request.conversation_id,
                run_id=request.run_id,
                configuration_id=request.configuration_id,
                answer=request.answer,
                interrupt_id=request.interrupt_id,
                profile=request.business_context.profile,
                body_state=request.business_context.body_state,
                relevant_history=request.business_context.relevant_history,
                episodic_notes=request.business_context.episodic_notes,
                current_diagnosis=request.business_context.current_diagnosis,
                current_treatment=request.business_context.current_treatment,
                recent_outcomes=request.business_context.recent_outcomes,
                spatial_context=(
                    request.business_context.spatial_context.model_dump(exclude_none=True)
                    if request.business_context.spatial_context
                    else None
                ),
                context_window=_context_window(request.business_context),
            ):
                yield serialize_runtime_event(event)
        except Exception:
            logger.exception("Error in runtime interrupt resume")
            yield serialize_runtime_event(
                ConsultationRuntimeEvent(
                    seq=1,
                    conversation_id=request.conversation_id,
                    run_id=request.run_id,
                    event=StreamErrorRuntimeEvent(message="Internal runtime resume error."),
                )
            )

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
