"""Runtime API routes for checkpointed consultation threads.

Learning path (Thought Forest note filenames):
- python-async-programming.md
- python-iterators-and-generators.md
- python-error-handling.md
- ndjson-sse-and-streaming-protocol-boundaries.md

Streaming turns and their E2E stubs live in ``runtime_turns`` and
``runtime_e2e``; this module keeps the data-lifecycle routes (checkpoint
cleanup, erasure) and re-exports the runtime surface other modules and tests
import.
"""

from __future__ import annotations

from typing import Any
from uuid import UUID

from fastapi import APIRouter, HTTPException

from ...runtime.checkpoint_retention import (
    CheckpointCleanupError,
    UnsupportedCheckpointerError,
    cleanup_thread_checkpoints,
)
from ...runtime.consultation_thread import get_runtime_graph
from ...runtime.erasure import ErasureFingerprint, RuntimeErasureReceipt, erase_runtime_copies
from . import runtime_turns
from .runtime_e2e import (
    _e2e_intake_tool_id,
    _e2e_safety_screen_question,
    _e2e_stub_enabled,
    _stub_event,
    e2e_resume_stream,
    e2e_turn_stream,
)
from .runtime_schemas import (
    AssetRuntimeErasureRequest,
    BusinessContext,
    CheckpointCleanupRequest,
    ConsultationRuntimeState,
    ContextWindow,
    ImageRef,
    ReferenceMotionContext,
    ResumeInterruptRequest,
    RuntimeErasureScope,
    SpatialContext,
    StartTurnRequest,
    StatementRuntimeErasureRequest,
    UserInput,
)
from .runtime_turns import _context_window, resume_interrupt, start_turn

__all__ = [
    "AssetRuntimeErasureRequest",
    "BusinessContext",
    "CheckpointCleanupRequest",
    "ConsultationRuntimeState",
    "ContextWindow",
    "ImageRef",
    "ReferenceMotionContext",
    "ResumeInterruptRequest",
    "RuntimeErasureScope",
    "SpatialContext",
    "StartTurnRequest",
    "StatementRuntimeErasureRequest",
    "UserInput",
    "_context_window",
    "_e2e_intake_tool_id",
    "_e2e_safety_screen_question",
    "_e2e_stub_enabled",
    "_erase_runtime",
    "_stub_event",
    "_validate_scope",
    "cleanup_checkpoints",
    "e2e_resume_stream",
    "e2e_turn_stream",
    "erase_asset_runtime",
    "erase_statement_runtime",
    "resume_interrupt",
    "router",
    "start_turn",
]

router = APIRouter(prefix="/runtime", tags=["runtime"])
router.include_router(runtime_turns.router)


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
