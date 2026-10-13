"""Versioned BodyFact extraction node; no persistent domain authority lives here."""

from __future__ import annotations

from typing import Any
from uuid import UUID

from langgraph.types import StreamWriter
from pydantic_ai.exceptions import AgentRunError

from ..ai.errors import GatewayUnavailableError
from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.body_fact_intake import body_fact_proposal
from ..services.body_fact_intake_service import BodyFactIntakeService


async def acquire_body_fact_turn(
    state: dict[str, Any],
    *,
    writer: StreamWriter,
    manifest: ConsultationAgentManifest,
    intake_service: BodyFactIntakeService | None = None,
) -> dict[str, Any]:
    text = str(state.get("latest_user_message") or "").strip()
    if not text:
        return {"intake_result": {}, "intake_question": None}
    body_state = state.get("body_state", {})
    if body_state.get("records_available") is not True:
        writer({"type": "text_delta", "delta": "身体记录暂时无法核验，本轮暂不自动保存。\n"})
        return {"intake_result": {"capture_status": "unavailable"}, "intake_question": None}
    try:
        run_id = str(UUID(str(state.get("run_id", ""))))
        output = await (intake_service or BodyFactIntakeService()).assess(
            latest_user_message=text,
            config=manifest,
        )
        proposal = body_fact_proposal(output, text, body_state.get("current_revision"))
    except (AgentRunError, GatewayUnavailableError, ValueError):
        # No heuristic fallback may turn a negative/hypothetical statement into
        # a positive symptom. The rest of the chat may proceed without saving.
        writer({"type": "text_delta", "delta": "本轮身体信息尚未保存，自动提取未通过核验。\n"})
        return {"intake_result": {"capture_status": "unavailable"}, "intake_question": None}

    result = {"turn_kind": output.turn_kind, "capture_status": "not_applicable"}
    if output.facts:
        tool_id = f"bodyfact-intake-v1:{run_id}"
        answer_scope = state.get("answer_interaction_id")
        if answer_scope:
            tool_id += f":answer:{UUID(str(answer_scope)).hex}"
        writer(
            {
                "type": "tool_call",
                "id": tool_id,
                "tool": "record_body_state",
                "args": {"fact_count": len(output.facts)},
            }
        )
        writer(
            {"type": "tool_result", "id": tool_id, "tool": "record_body_state", "result": proposal}
        )
        result["capture_status"] = "proposed"
    question = None
    if output.turn_kind == "ambiguous":
        question = {
            "question": output.clarification or "这条身体描述指的是你本人吗？",
            "reason": "clarify_body_fact_subject",
            "context": "只有明确的本人陈述才会写入你的身体记录。",
            "answer_type": "text",
            "allow_custom_input": True,
        }
    if output.turn_kind == "correction":
        writer(
            {
                "type": "text_delta",
                "delta": "这涉及更正既有记录，本轮没有把更正请求当作一条新事实保存。\n",
            }
        )
    return {"intake_result": result, "intake_question": question}


async def acquire_body_fact_answer(
    *,
    run_id: str,
    interrupt_id: str,
    answer: dict[str, Any],
    body_state: dict[str, Any],
    writer: StreamWriter,
    manifest: ConsultationAgentManifest,
    intake_service: BodyFactIntakeService | None = None,
) -> dict[str, Any]:
    """Extract literal self-statements only after an exact Go-accepted answer.

    A short answer such as yes is not expanded into the assistant's medical
    suggestion. Question-dependent interpretation needs its own typed contract.
    """
    UUID(interrupt_id)
    text = answer.get("text")
    if not isinstance(text, str) or not text.strip():
        return {"capture_status": "not_applicable"}
    acquired = await acquire_body_fact_turn(
        {
            "run_id": run_id,
            "answer_interaction_id": interrupt_id,
            "latest_user_message": text,
            "body_state": body_state,
        },
        writer=writer,
        manifest=manifest,
        intake_service=intake_service,
    )
    return acquired["intake_result"]
