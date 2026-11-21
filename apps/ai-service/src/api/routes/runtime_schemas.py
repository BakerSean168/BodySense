"""Request and context models for the runtime API routes."""

from __future__ import annotations

from typing import Any, Literal
from uuid import UUID

from pydantic import BaseModel, Field


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
    # S4 real-time recall: non-authoritative narrative notes with provenance.
    # Absent/empty is today's behavior. Never used by Diagnosis, Safety or
    # TrainingPlan inputs.
    episodic_notes: list[dict[str, Any]] = Field(default_factory=list, max_length=8)
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


class CheckpointCleanupRequest(BaseModel):
    """Go asks to drop the checkpoint copies of Runs it is about to prune."""

    conversation_id: UUID
    user_id: UUID
    pruned_run_ids: list[UUID] = Field(max_length=10_000)


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
