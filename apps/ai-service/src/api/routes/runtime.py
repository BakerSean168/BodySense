"""Runtime API routes for checkpointed consultation threads.

Learning path (Thought Forest note filenames):
- python-async-programming.md
- python-iterators-and-generators.md
- python-error-handling.md
- ndjson-sse-and-streaming-protocol-boundaries.md

The nested async generators bridge domain events to an HTTP byte stream. They
yield one JSON record at a time instead of materializing the whole reply.
"""

from __future__ import annotations

import json
import logging
from typing import Any

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from ...runtime.consultation_thread import resume_thread_interrupt, stream_thread_turn

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/runtime", tags=["runtime"])


class ImageRef(BaseModel):
    """Server-resolved image for multimodal turns (data URL, never raw client URL)."""

    upload_id: str | None = None
    mime_type: str | None = None
    data_url: str


class UserInput(BaseModel):
    type: str = "user_message"
    text: str
    images: list[ImageRef] = Field(default_factory=list)


class ConsultationSnapshot(BaseModel):
    phase: str = "collecting"
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)


class BusinessContext(BaseModel):
    profile: dict[str, Any] = Field(default_factory=dict)
    consultation_snapshot: ConsultationSnapshot = Field(default_factory=ConsultationSnapshot)
    # Prefetched completed posture analysis from Go (user_uploads.analysis_result).
    posture_analysis: dict[str, Any] | None = None


class StartTurnRequest(BaseModel):
    run_id: str
    conversation_id: str
    user_id: str
    input: UserInput
    business_context: BusinessContext = Field(default_factory=BusinessContext)


class ResumeInterruptRequest(BaseModel):
    run_id: str
    conversation_id: str
    user_id: str
    interrupt_id: str
    answer: dict[str, Any]
    business_context: BusinessContext = Field(default_factory=BusinessContext)


@router.post("/threads/{thread_id}/turns")
async def start_turn(thread_id: str, request: StartTurnRequest):
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
                images=[img.model_dump() for img in request.input.images],
                profile=request.business_context.profile,
                extracted_info=request.business_context.consultation_snapshot.extracted_info,
                phase=request.business_context.consultation_snapshot.phase,
                posture_analysis=request.business_context.posture_analysis,
            ):
                # NDJSON uses a real newline as the record boundary. Any newline
                # inside a JSON string is escaped by json.dumps.
                yield json.dumps(event.model_dump(exclude_none=True), ensure_ascii=False) + "\n"
        except Exception:
            # The HTTP headers may already be sent, so raising an HTTPException
            # cannot reliably replace the response. Emit a protocol-level error
            # record and log the original exception server-side instead.
            logger.exception("Error in runtime thread turn")
            yield json.dumps(
                {
                    "version": 1,
                    "seq": 1,
                    "channel": "stream",
                    "type": "stream.error",
                    "ids": {"run_id": request.run_id, "conversation_id": request.conversation_id},
                    "payload": {"message": "Internal runtime error."},
                },
                ensure_ascii=False,
            ) + "\n"

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        # Both headers reduce intermediary buffering so clients observe records
        # close to the time they are yielded.
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )


@router.post("/threads/{thread_id}/interrupts/{interrupt_id}/resume")
async def resume_interrupt(thread_id: str, interrupt_id: str, request: ResumeInterruptRequest):
    async def ndjson_generator():
        try:
            async for event in resume_thread_interrupt(
                thread_id=thread_id,
                conversation_id=request.conversation_id,
                run_id=request.run_id,
                answer=request.answer,
            ):
                yield json.dumps(event.model_dump(exclude_none=True), ensure_ascii=False) + "\n"
        except Exception:
            logger.exception("Error in runtime interrupt resume")
            yield json.dumps(
                {
                    "version": 1,
                    "seq": 1,
                    "channel": "stream",
                    "type": "stream.error",
                    "ids": {
                        "run_id": request.run_id,
                        "conversation_id": request.conversation_id,
                        "interaction_id": interrupt_id,
                    },
                    "payload": {"message": "Internal runtime resume error."},
                },
                ensure_ascii=False,
            ) + "\n"

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
