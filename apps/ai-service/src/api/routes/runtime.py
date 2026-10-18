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
import logging
import os
from typing import Any, Literal
from uuid import NAMESPACE_URL, UUID, uuid5

from fastapi import APIRouter, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from ...models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    ConsultationRuntimeEvent,
    ConsultationRuntimeEventPayload,
    InteractionRequiredRuntimeEvent,
    PhaseChangedRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamDoneRuntimeEvent,
    StreamErrorRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
)
from ...runtime.checkpoint_retention import (
    CheckpointCleanupError,
    UnsupportedCheckpointerError,
    cleanup_thread_checkpoints,
)
from ...runtime.consultation_thread import (
    get_consultation_manifest,
    get_runtime_graph,
    resume_thread_interrupt,
    stream_thread_turn,
)
from ...runtime.erasure import ErasureFingerprint, RuntimeErasureReceipt, erase_runtime_copies
from ..runtime_proto_adapter import (
    RuntimeCommandError,
    parse_resume_interrupt_command,
    parse_start_turn_command,
    serialize_runtime_event,
)

logger = logging.getLogger(__name__)


async def _e2e_safety_screen_question(question: dict[str, Any]) -> tuple[str, dict[str, Any]]:
    """Normalize the production intake question exactly as the runtime does."""
    from ...services.agent.tools.ask_user import handle_ask_user

    normalized = await handle_ask_user(question)
    if not isinstance(normalized.content, dict) or "state_binding" not in normalized.content:
        raise RuntimeError("E2E symptom intake was not normalized")
    return f"intake-{normalized.content['state_binding']['capture_id']}", normalized.content


def _e2e_intake_tool_id(run_id: str, answer: dict[str, Any]) -> str:
    """The pending E2E interrupt's tool call: the safety screen for a form answer."""
    from ...testing_support.body_state_suggestion_fixture import SUGGESTION_ANSWER_OPTIONS

    selected = answer.get("selected")
    if (
        isinstance(selected, list)
        and len(selected) == 1
        and selected[0] in SUGGESTION_ANSWER_OPTIONS
    ):
        from ...runtime.body_state_suggestion import suggestion_tool_call_id

        return suggestion_tool_call_id(run_id)
    if not isinstance(answer.get("fields"), dict):
        return f"ask-bodyfact:{run_id}"
    from ...services.consultation_state_acquisition import safety_screen_capture_id

    return f"intake-{safety_screen_capture_id(f'bodyfact-intake-v1:{run_id}', 0)}"


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
    run_id: str,
    conversation_id: str,
    event: ConsultationRuntimeEventPayload,
    tool_call_id: str | None = None,
) -> str:
    return serialize_runtime_event(
        ConsultationRuntimeEvent(
            seq=seq,
            conversation_id=conversation_id,
            run_id=run_id,
            event=event,
            tool_call_id=tool_call_id,
        )
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
    # Go's persisted id of this user message (the only citable message).
    message_id: str | None = None


class ConsultationRuntimeState(BaseModel):
    phase: str = "collecting"
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)


class ReferenceMotionContext(BaseModel):
    id: str = Field(min_length=1, max_length=80)
    label: str = Field(min_length=1, max_length=120)
    phase: float = Field(ge=0, le=1)
    paused: bool
    source: Literal["reference_animation"]


class SpatialContext(BaseModel):
    body_region_id: str | None = None
    body_region_label: str | None = None
    body_region_ids: list[str] = Field(default_factory=list, max_length=35)
    anatomy_id: str | None = None
    anatomy_name: str | None = None
    reference_motion: ReferenceMotionContext | None = None


class ContextWindow(BaseModel):
    """Go's chat retention context cursor (PC-RET-001)."""

    context_start_seq: int = Field(ge=1)
    run_ids: list[str] = Field(default_factory=list, max_length=4096)


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
    spatial_context: SpatialContext | None = None
    # Prefetched completed posture analysis from Go (user_uploads.analysis_result).
    posture_analysis: dict[str, Any] | None = None
    # Absent when Go could not read it; the runtime then uses its fallback.
    context_window: ContextWindow | None = None


class StartTurnRequest(BaseModel):
    run_id: str
    conversation_id: str
    user_id: str
    configuration_id: str = Field(min_length=1)
    input: UserInput
    business_context: BusinessContext = Field(default_factory=BusinessContext)


class ResumeInterruptRequest(BaseModel):
    run_id: str
    conversation_id: str
    user_id: str
    configuration_id: str = Field(min_length=1)
    interrupt_id: str
    answer: dict[str, Any]
    business_context: BusinessContext = Field(default_factory=BusinessContext)


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

        async def e2e_generator():
            if "E2E_HOLD_RUN_LONG" in request.input.text:
                await asyncio.sleep(15)
            elif "E2E_HOLD_RUN" in request.input.text:
                await asyncio.sleep(0.75)
            trigger_safety = "E2E_TRIGGER_SAFETY" in request.input.text
            seq = 1
            manifest = get_consultation_manifest(request.configuration_id)
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=AgentConfigurationRuntimeEvent(
                    agent_configuration=manifest.provenance(),
                    execution_provenance={
                        "status": "executed",
                        "runtime": "langgraph-e2e-stub",
                        "logical_model": manifest.logical_model,
                        "model_group_revision": manifest.model_group_revision,
                        "usage": {},
                    },
                ),
            )
            seq += 1
            intake_question: dict[str, Any] | None = None
            if manifest.intake and manifest.intake.policy_revision == "body-fact-intake-v1":
                from ...testing_support.body_fact_fixture import body_fact_fixture_events

                fixture_events, turn_question = await body_fact_fixture_events(
                    request.input.text,
                    request.business_context.body_state,
                    manifest,
                    request.run_id,
                )
                if turn_question and turn_question.get("purpose") == "symptom_intake":
                    intake_question = turn_question
                for item in fixture_events:
                    if item["type"] == "tool_call":
                        event = ToolCallRuntimeEvent(tool=item["tool"], args=item["args"])
                    elif item["type"] == "tool_result":
                        event = ToolResultRuntimeEvent(tool=item["tool"], result=item["result"])
                    else:
                        event = TextDeltaRuntimeEvent(delta=item["delta"])
                    yield _stub_event(
                        seq=seq,
                        run_id=request.run_id,
                        conversation_id=request.conversation_id,
                        event=event,
                        tool_call_id=item.get("id"),
                    )
                    seq += 1
            suggestion: dict[str, Any] | None = None
            if (
                manifest.intake
                and manifest.intake.policy_revision == "body-fact-intake-v1"
                and intake_question is None
            ):
                from ...runtime.body_state_suggestion import suggestion_tool_call_id
                from ...testing_support.body_state_suggestion_fixture import (
                    body_state_suggestion_fixture,
                )

                suggestion = await body_state_suggestion_fixture(
                    request.input.text,
                    request.business_context.body_state,
                    manifest,
                    request.run_id,
                    request.input.message_id,
                    pending_revisions=int(
                        any(item["type"] == "tool_result" for item in fixture_events)
                    ),
                )
            if suggestion is not None:
                # The production proposer's typed suggestion; Go binds it to the
                # cited user message or replaces it with an unverified notice.
                tool_id = suggestion_tool_call_id(request.run_id)
                placeholder = {
                    "question": "要把你说过的这句话记录下来吗？",
                    "answer_type": "single_choice",
                    "options": ["记录", "不记录"],
                    "allow_custom_input": False,
                    "required": True,
                }
                yield _stub_event(
                    seq=seq,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    tool_call_id=tool_id,
                    event=ToolCallRuntimeEvent(tool="ask_user", args=placeholder),
                )
                yield _stub_event(
                    seq=seq + 1,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    tool_call_id=tool_id,
                    event=InteractionRequiredRuntimeEvent(
                        interaction_id=uuid5(
                            NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}"
                        ).hex,
                        question=placeholder,
                        suggestion=suggestion,
                    ),
                )
                return
            if intake_question is not None or request.input.text in {
                "E2E_BODYFACT_ASK_SELF",
                "E2E_BODYFACT_ASK_CHOICE",
            }:
                native_id = uuid5(NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}").hex
                tool_id = f"ask-bodyfact:{request.run_id}"
                choice = request.input.text.endswith("CHOICE")
                question: dict[str, Any] = {
                    "question": "Which side?" if choice else "Describe your own current symptoms.",
                    "answer_type": "single_choice" if choice else "text",
                    "allow_custom_input": False if choice else True,
                }
                if choice:
                    question["options"] = ["left", "right"]
                if intake_question is not None:
                    # The production node asked the safety screen for the
                    # symptom it just proposed; Go binds it to the commit.
                    tool_id, question = await _e2e_safety_screen_question(intake_question)
                yield _stub_event(
                    seq=seq,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    tool_call_id=tool_id,
                    event=ToolCallRuntimeEvent(tool="ask_user", args=question),
                )
                yield _stub_event(
                    seq=seq + 1,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    tool_call_id=tool_id,
                    event=InteractionRequiredRuntimeEvent(
                        interaction_id=native_id, question=question
                    ),
                )
                return
            if trigger_safety:
                yield _stub_event(
                    seq=seq,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    event=RedFlagDetectedRuntimeEvent(
                        has_red_flags=True,
                        flags=[
                            {
                                "category": "weakness",
                                "severity": "high",
                                "message": "E2E deterministic safety signal",
                            }
                        ],
                    ),
                )
                seq += 1
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=TextDeltaRuntimeEvent(delta="E2E consultation completed."),
            )
            seq += 1
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=PhaseChangedRuntimeEvent(
                    from_phase=request.business_context.runtime_state.phase,
                    to="ready_for_analysis",
                    reason="e2e deterministic completion",
                ),
            )
            seq += 1
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=StreamDoneRuntimeEvent(
                    usage={"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}
                ),
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
                user_message_id=request.input.message_id,
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

        async def e2e_resume_generator():
            expected = uuid5(NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}")
            if UUID(request.interrupt_id) != expected:
                yield _stub_event(
                    seq=1,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    event=StreamErrorRuntimeEvent(message="Synthetic native interrupt mismatch."),
                )
                return
            manifest = get_consultation_manifest(request.configuration_id)
            seq = 1
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=AgentConfigurationRuntimeEvent(
                    agent_configuration=manifest.provenance(),
                    execution_provenance={
                        "status": "executed",
                        "runtime": "langgraph-e2e-stub",
                        "logical_model": manifest.logical_model,
                        "model_group_revision": manifest.model_group_revision,
                        "usage": {},
                    },
                ),
            )
            seq += 1
            from ...testing_support.body_fact_fixture import body_fact_fixture_events

            # A safety-screen form answer is not a free-text statement; like the
            # production resume, it skips statement extraction.
            answer_events: list[dict[str, Any]] = []
            if not isinstance(request.answer.get("fields"), dict):
                answer_events, _ = await body_fact_fixture_events(
                    str(request.answer.get("text", "")),
                    request.business_context.body_state,
                    manifest,
                    request.run_id,
                    interrupt_id=request.interrupt_id,
                )
            for item in answer_events:
                if item["type"] == "tool_call":
                    event = ToolCallRuntimeEvent(tool=item["tool"], args=item["args"])
                elif item["type"] == "tool_result":
                    event = ToolResultRuntimeEvent(tool=item["tool"], result=item["result"])
                else:
                    event = TextDeltaRuntimeEvent(delta=item["delta"])
                yield _stub_event(
                    seq=seq,
                    run_id=request.run_id,
                    conversation_id=request.conversation_id,
                    event=event,
                    tool_call_id=item.get("id"),
                )
                seq += 1
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                tool_call_id=_e2e_intake_tool_id(request.run_id, request.answer),
                event=ToolResultRuntimeEvent(tool="ask_user", result={"answer": request.answer}),
            )
            yield _stub_event(
                seq=seq + 1,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=TextDeltaRuntimeEvent(delta="E2E accepted answer processed."),
            )
            yield _stub_event(
                seq=seq + 2,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=StreamDoneRuntimeEvent(),
            )

        return StreamingResponse(e2e_resume_generator(), media_type="application/x-ndjson")

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


class CheckpointCleanupRequest(BaseModel):
    """Go asks to drop the checkpoint copies of Runs it is about to prune."""

    conversation_id: UUID
    user_id: UUID
    pruned_run_ids: list[UUID] = Field(max_length=10_000)


@router.post("/threads/{thread_id}/checkpoints/cleanup")
async def cleanup_checkpoints(thread_id: UUID, request: CheckpointCleanupRequest):
    # The consultation thread is the Conversation (Go passes its id as the
    # thread id), so a mismatch is a caller bug, never a different thread.
    if thread_id != request.conversation_id:
        raise HTTPException(
            status_code=422,
            detail={"code": "thread_mismatch", "message": "thread is not the conversation"},
        )
    try:
        receipt = await cleanup_thread_checkpoints(
            await get_runtime_graph(),
            thread_id=str(thread_id),
            pruned_run_ids=[str(run_id) for run_id in request.pruned_run_ids],
        )
    except UnsupportedCheckpointerError as exc:
        raise HTTPException(status_code=503, detail={"code": exc.code, "message": str(exc)})
    except CheckpointCleanupError as exc:
        raise HTTPException(status_code=409, detail={"code": exc.code, "message": str(exc)})
    return {
        "status": receipt.status,
        "thread_id": receipt.thread_id,
        "pruned_run_ids": receipt.pruned_run_ids,
        "checkpoint_id": receipt.checkpoint_id,
        "removed_messages": receipt.removed_messages,
        "retained_messages": receipt.retained_messages,
        "deleted_checkpoints": receipt.deleted_checkpoints,
        "remaining_run_ids": receipt.remaining_run_ids,
    }


class RuntimeErasureScope(BaseModel):
    user_id: UUID
    # Every consultation thread (= Conversation) of the user, and the subset
    # the source entered through (an attaching message, a stated message).
    thread_ids: list[UUID] = Field(max_length=1_000)
    attached_thread_ids: list[UUID] = Field(default_factory=list, max_length=1_000)


class AssetRuntimeErasureRequest(RuntimeErasureScope):
    """Go asks to erase one asset from the user's consultation threads (AS-002)."""

    # The asset's content hash: inline image copies are matched by the
    # SHA-256 of their decoded bytes, derived copies by the asset id.
    content_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class StatementRuntimeErasureRequest(RuntimeErasureScope):
    """Go asks to erase one user statement and the BodyState records derived
    from it (their ids) from the user's consultation threads (AS-002)."""

    record_ids: list[UUID] = Field(default_factory=list, max_length=2_000)


def _validate_scope(scope: RuntimeErasureScope) -> None:
    if not set(scope.attached_thread_ids) <= set(scope.thread_ids):
        raise HTTPException(
            status_code=422,
            detail={"code": "thread_mismatch", "message": "attached threads must be listed"},
        )


async def _erase_runtime(
    subject_id: UUID, fingerprint: ErasureFingerprint, scope: RuntimeErasureScope
) -> dict[str, Any]:
    _validate_scope(scope)
    try:
        receipt: RuntimeErasureReceipt = await erase_runtime_copies(
            await get_runtime_graph(),
            subject_id=str(subject_id),
            fingerprint=fingerprint,
            thread_ids=[str(thread_id) for thread_id in scope.thread_ids],
            attached_thread_ids=[str(thread_id) for thread_id in scope.attached_thread_ids],
        )
    except UnsupportedCheckpointerError as exc:
        raise HTTPException(status_code=503, detail={"code": exc.code, "message": str(exc)})
    return {
        "status": receipt.status,
        "subject_id": receipt.subject_id,
        "threads": [
            {
                "thread_id": item.thread_id,
                "status": item.status,
                "checkpoint_id": item.checkpoint_id,
                "redacted_refs": item.redacted_refs,
                "deleted_checkpoints": item.deleted_checkpoints,
                "failure_code": item.failure_code,
            }
            for item in receipt.threads
        ],
    }


@router.post("/assets/{asset_id}/erasure")
async def erase_asset_runtime(asset_id: UUID, request: AssetRuntimeErasureRequest):
    fingerprint = ErasureFingerprint(ids=(str(asset_id),), content_sha256=request.content_sha256)
    return await _erase_runtime(asset_id, fingerprint, request)


@router.post("/statements/{statement_id}/erasure")
async def erase_statement_runtime(statement_id: UUID, request: StatementRuntimeErasureRequest):
    ids = (str(statement_id), *dict.fromkeys(str(record_id) for record_id in request.record_ids))
    return await _erase_runtime(statement_id, ErasureFingerprint(ids=ids), request)
