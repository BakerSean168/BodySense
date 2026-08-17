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

import asyncio
import json
import logging
import os
from typing import Any

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from ...runtime.consultation_thread import resume_thread_interrupt, stream_thread_turn

logger = logging.getLogger(__name__)


def _e2e_stub_enabled() -> bool:
    environment = os.getenv("ENVIRONMENT", "development").lower()
    return os.getenv("BODYSENSE_E2E_STUB_AI") == "1" and environment in {
        "development",
        "test",
        "e2e",
    }


def _stub_event(
    *,
    seq: int,
    channel: str,
    event_type: str,
    run_id: str,
    conversation_id: str,
    payload: dict[str, Any],
) -> str:
    return (
        json.dumps(
            {
                "version": 1,
                "seq": seq,
                "channel": channel,
                "type": event_type,
                "ids": {
                    "conversation_id": conversation_id,
                    "run_id": run_id,
                    "turn_id": None,
                    "message_id": None,
                    "tool_call_id": None,
                    "interaction_id": None,
                    "job_id": None,
                },
                "payload": payload,
            },
            ensure_ascii=False,
        )
        + "\n"
    )


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


class ConsultationRuntimeState(BaseModel):
    phase: str = "collecting"
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)


class BusinessContext(BaseModel):
    profile: dict[str, Any] = Field(default_factory=dict)
    # Go-owned BodyState is durable health truth. Runtime state is limited to
    # transient consultation orchestration such as extraction and phase.
    body_state: dict[str, Any] = Field(default_factory=dict)
    runtime_state: ConsultationRuntimeState = Field(default_factory=ConsultationRuntimeState)
    relevant_history: list[dict[str, Any]] = Field(default_factory=list)
    current_diagnosis: dict[str, Any] = Field(default_factory=dict)
    current_treatment: dict[str, Any] = Field(default_factory=dict)
    recent_outcomes: list[dict[str, Any]] = Field(default_factory=list)
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
    if _e2e_stub_enabled():

        async def e2e_generator():
            if "E2E_HOLD_RUN" in request.input.text:
                await asyncio.sleep(0.75)
            trigger_safety = "E2E_TRIGGER_SAFETY" in request.input.text
            seq = 1
            if trigger_safety:
                yield _stub_event(
                    seq=seq,
                    channel="safety",
                    event_type="safety.red_flag.detected",
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    payload={
                        "has_red_flags": True,
                        "flags": [
                            {
                                "type": "weakness",
                                "severity": "high",
                                "message": "E2E deterministic safety signal",
                            }
                        ],
                    },
                )
                seq += 1
            yield _stub_event(
                seq=seq,
                channel="message",
                event_type="message.text.delta",
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                payload={"delta": "E2E consultation completed."},
            )
            seq += 1
            yield _stub_event(
                seq=seq,
                channel="state",
                event_type="state.phase.changed",
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                payload={
                    "from": request.business_context.runtime_state.phase,
                    "to": "ready_for_analysis",
                    "reason": "e2e deterministic completion",
                },
            )
            seq += 1
            yield _stub_event(
                seq=seq,
                channel="stream",
                event_type="stream.done",
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                payload={"usage": {"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}},
            )

        return StreamingResponse(e2e_generator(), media_type="application/x-ndjson")

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
                body_state=request.business_context.body_state,
                extracted_info=request.business_context.runtime_state.extracted_info,
                phase=request.business_context.runtime_state.phase,
                posture_analysis=request.business_context.posture_analysis,
                relevant_history=request.business_context.relevant_history,
                current_diagnosis=request.business_context.current_diagnosis,
                current_treatment=request.business_context.current_treatment,
                recent_outcomes=request.business_context.recent_outcomes,
            ):
                # NDJSON uses a real newline as the record boundary. Any newline
                # inside a JSON string is escaped by json.dumps.
                yield json.dumps(event.model_dump(exclude_none=True), ensure_ascii=False) + "\n"
        except Exception:
            # The HTTP headers may already be sent, so raising an HTTPException
            # cannot reliably replace the response. Emit a protocol-level error
            # record and log the original exception server-side instead.
            logger.exception("Error in runtime thread turn")
            yield (
                json.dumps(
                    {
                        "version": 1,
                        "seq": 1,
                        "channel": "stream",
                        "type": "stream.error",
                        "ids": {
                            "run_id": request.run_id,
                            "conversation_id": request.conversation_id,
                        },
                        "payload": {"message": "Internal runtime error."},
                    },
                    ensure_ascii=False,
                )
                + "\n"
            )

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
                profile=request.business_context.profile,
                body_state=request.business_context.body_state,
                relevant_history=request.business_context.relevant_history,
                current_diagnosis=request.business_context.current_diagnosis,
                current_treatment=request.business_context.current_treatment,
                recent_outcomes=request.business_context.recent_outcomes,
            ):
                yield json.dumps(event.model_dump(exclude_none=True), ensure_ascii=False) + "\n"
        except Exception:
            logger.exception("Error in runtime interrupt resume")
            yield (
                json.dumps(
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
                )
                + "\n"
            )

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
