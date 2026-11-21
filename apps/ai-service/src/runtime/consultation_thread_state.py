"""Consultation thread state shape and per-turn preparation."""

from __future__ import annotations

from typing import Annotated, Any, TypedDict

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from .checkpoint_retention import tag_runtime_message


def _merge_symptoms(
    existing: list[dict[str, Any]],
    new: list[dict[str, Any]],
) -> list[dict[str, Any]]:
    by_part: dict[str, dict[str, Any]] = {}
    for symptom in existing:
        part = symptom.get("body_part", "")
        if part:
            by_part[part] = dict(symptom)
    for symptom in new:
        part = symptom.get("body_part", "")
        if not part:
            continue
        if part in by_part:
            by_part[part].update(symptom)
        else:
            by_part[part] = dict(symptom)
    return list(by_part.values())


class ConsultationThreadState(TypedDict, total=False):
    session_id: str
    run_id: str
    user_id: str
    profile: dict[str, Any]
    # Go-owned durable longitudinal health state. This is business truth;
    # extracted_symptoms remains a migration/runtime convenience only.
    body_state: dict[str, Any]
    relevant_history: list[dict[str, Any]]
    # S4 real-time recall: bounded non-authoritative narrative notes with
    # provenance. Presentation context only, never clinical evidence.
    episodic_notes: list[dict[str, Any]]
    current_diagnosis: dict[str, Any]
    current_treatment: dict[str, Any]
    recent_outcomes: list[dict[str, Any]]
    spatial_context: dict[str, Any]
    phase: str
    current_user_message: str
    latest_user_message: str
    # Go's persisted id of the turn's user message (the only citable message).
    current_user_message_id: str
    latest_user_message_id: str
    pending_user_images: list[dict[str, Any]]
    runtime_messages: list[dict[str, Any]]
    # Go's chat retention context cursor for this turn (PC-RET-001):
    # {"context_start_seq": int, "run_ids": [str, ...]} or None.
    context_window: dict[str, Any] | None
    extracted_symptoms: Annotated[list[dict[str, Any]], _merge_symptoms]
    red_flag_result: dict[str, Any] | None
    accumulated_text: str
    pending_tool_calls: list[dict[str, Any]]
    tool_rounds: int
    llm_available: bool
    diagnosis_result: dict[str, Any] | None
    treatment_result: dict[str, Any] | None
    # Prefetched by Go from user_uploads.analysis_result (Phase 3-B1 / P4).
    posture_analysis: dict[str, Any] | None
    # North-Star: resolved immutable Agent configuration for this turn.
    consultation_manifest: ConsultationAgentManifest
    retrieved_published_evidence: dict[str, dict[str, Any]]
    answer_attributions: list[dict[str, Any]]
    intake_result: dict[str, Any]
    intake_question: dict[str, Any] | None
    # BS-AGENT-001: fact batches this stream proposed since Go last sent
    # body_state (Go commits each one as one revision before continuing), and
    # the one typed suggestion awaiting the user's confirmation.
    body_state_pending_revisions: int
    body_state_suggestion: dict[str, Any] | None


def _user_content_with_images(
    text: str,
    images: list[dict[str, Any]] | None,
) -> str | list[dict[str, Any]]:
    """Build OpenAI-compatible multimodal user content when images are present.

    Text-only turns stay plain strings so existing prompts/tests keep working.
    Image turns use the content-block list form already supported by ChatMessage.
    """
    cleaned = (text or "").strip()
    refs = [img for img in (images or []) if isinstance(img, dict) and img.get("data_url")]
    if not refs:
        return cleaned

    blocks: list[dict[str, Any]] = []
    if cleaned:
        blocks.append({"type": "text", "text": cleaned})
    # Cap mirrors Go-side maxImages=3.
    for img in refs[:3]:
        data_url = str(img["data_url"])
        # Basic shape guard: only accept data:image/* URLs resolved by Go.
        if not data_url.startswith("data:image/"):
            continue
        blocks.append(
            {
                "type": "image_url",
                "image_url": {"url": data_url},
            }
        )
    if not blocks:
        return cleaned or "（用户上传了照片，但未能读取图像内容）"
    if len(blocks) == 1 and blocks[0].get("type") == "text":
        return str(blocks[0].get("text") or cleaned)
    return blocks


async def prepare_turn(state: ConsultationThreadState) -> dict[str, Any]:
    current_user_message = state.get("current_user_message", "").strip()
    pending_images = state.get("pending_user_images") or []
    if not current_user_message and not pending_images:
        return {}

    content = _user_content_with_images(current_user_message, pending_images)
    runtime_messages = list(state.get("runtime_messages", []))
    # The run_id tag lets retention drop this Run's segment once Go prunes it.
    runtime_messages.append(
        tag_runtime_message({"role": "user", "content": content}, state.get("run_id"))
    )
    return {
        "runtime_messages": runtime_messages,
        "latest_user_message": current_user_message,
        "latest_user_message_id": state.get("current_user_message_id") or "",
        "current_user_message": "",
        "current_user_message_id": "",
        "body_state_pending_revisions": 0,
        "body_state_suggestion": None,
        "pending_user_images": [],
        "pending_tool_calls": [],
        "accumulated_text": "",
        "tool_rounds": 0,
        "retrieved_published_evidence": {},
        "answer_attributions": [],
        "intake_result": {},
        "intake_question": None,
    }
