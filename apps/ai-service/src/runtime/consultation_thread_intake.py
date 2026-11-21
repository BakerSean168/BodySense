"""Turn classification and structured intake nodes."""

from __future__ import annotations

import json
from typing import Any

from langgraph.types import StreamWriter, interrupt

from ..models.body_fact_intake import BODY_FACT_INTAKE_POLICY
from ..services.agent.consultation_tools import get_consultation_executor
from ..services.agent.tool_types import ToolStatus
from ..services.consultation_intake_service import ConsultationIntakeService
from ..services.consultation_state_acquisition import (
    apply_structured_intake_answer,
    build_symptom_intake_question,
    intake_state_candidates,
)
from .body_fact_capture import acquire_body_fact_turn
from .body_state_suggestion import (
    confirm_body_state_suggestion,
    propose_body_state_suggestion,
)
from .consultation_thread_manifest import get_consultation_manifest
from .consultation_thread_state import ConsultationThreadState

_intake_service_instance: ConsultationIntakeService | None = None


def _get_intake_service() -> ConsultationIntakeService:
    global _intake_service_instance
    if _intake_service_instance is None:
        _intake_service_instance = ConsultationIntakeService()
    return _intake_service_instance


async def acquire_turn_state(
    state: ConsultationThreadState,
    *,
    writer: StreamWriter,
) -> dict[str, Any]:
    """Classify the latest turn and emit durable candidates before any prose."""
    manifest = state.get("consultation_manifest") or get_consultation_manifest(None)
    latest_user_message = state.get("latest_user_message", "").strip()
    if manifest.intake is None or not latest_user_message:
        return {"intake_result": {}, "intake_question": None}

    if manifest.intake.policy_revision == BODY_FACT_INTAKE_POLICY:
        acquired = await acquire_body_fact_turn(dict(state), writer=writer, manifest=manifest)
        proposed = acquired["intake_result"].get("capture_status") == "proposed"
        return {**acquired, "body_state_pending_revisions": 1 if proposed else 0}

    output = await _get_intake_service().assess(
        latest_user_message=latest_user_message,
        profile=state.get("profile", {}),
        body_state=state.get("body_state", {}),
        config=manifest,
    )
    symptoms, lifestyle = intake_state_candidates(
        output,
        run_id=state.get("run_id", ""),
        latest_user_message=latest_user_message,
    )
    for symptom in symptoms:
        writer({"type": "extracted_info", "info": symptom})
    for item in lifestyle:
        writer({"type": "lifestyle_context", "context": item})

    question = build_symptom_intake_question(symptoms[0]) if symptoms else None
    result = output.model_dump(mode="json")
    result["symptoms"] = symptoms
    result["lifestyle"] = lifestyle
    return {
        "intake_result": result,
        "intake_question": question,
        "extracted_symptoms": symptoms,
    }


async def enforce_state_acquisition(
    state: ConsultationThreadState,
    *,
    writer: StreamWriter,
) -> dict[str, Any]:
    """Pause on one deterministic structured intake gap before visible advice."""
    question = state.get("intake_question")
    if not question or state.get("red_flag_result"):
        return {"intake_question": None}

    binding = question.get("state_binding")
    capture_id = str(binding.get("capture_id") if isinstance(binding, dict) else "")
    tool_call_id = f"intake-{capture_id or state.get('run_id', 'clarification')}"
    writer(
        {
            "type": "tool_call",
            "id": tool_call_id,
            "tool": "ask_user",
            "args": question,
        }
    )
    result = await get_consultation_executor().execute(
        tool_call_id,
        "ask_user",
        question,
    )
    if result.status != ToolStatus.INTERRUPTED or not isinstance(result.content, dict):
        raise RuntimeError(result.error or "state acquisition question could not be normalized")

    normalized_question = result.content
    answer = interrupt(
        {
            "interrupt_type": "ask_user",
            "tool_call_id": tool_call_id,
            "tool_name": "ask_user",
            "question": normalized_question,
        }
    )
    writer(
        {
            "type": "tool_result",
            "id": tool_call_id,
            "tool": "ask_user",
            "result": {"answer": answer},
        }
    )

    completed = apply_structured_intake_answer(normalized_question, answer)
    runtime_messages = list(state.get("runtime_messages", []))
    runtime_messages.append(
        {
            "role": "user",
            "content": "[结构化补充回答] " + json.dumps(answer, ensure_ascii=False),
        }
    )
    update: dict[str, Any] = {
        "runtime_messages": runtime_messages,
        "intake_question": None,
    }
    if completed is not None:
        update["extracted_symptoms"] = [completed]
    return update


async def propose_suggestion(state: ConsultationThreadState) -> dict[str, Any]:
    manifest = state.get("consultation_manifest") or get_consultation_manifest(None)
    return await propose_body_state_suggestion(dict(state), manifest=manifest)


async def confirm_suggestion(
    state: ConsultationThreadState, *, writer: StreamWriter
) -> dict[str, Any]:
    return await confirm_body_state_suggestion(dict(state), writer=writer)
