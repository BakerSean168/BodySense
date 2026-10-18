"""Graph nodes that emit at most one source-cited BodyState suggestion per turn.

``propose_body_state_suggestion`` runs the bounded model proposer once and keeps
the typed suggestion in state; ``confirm_body_state_suggestion`` pauses on it.
They are separate nodes because LangGraph re-executes an interrupted node on
resume, and the proposer must not run twice. Go binds the suggestion to the
cited message and commits only after the user confirms.
"""

from __future__ import annotations

import json
import logging
from typing import Any
from uuid import UUID

from langgraph.types import StreamWriter, interrupt
from pydantic_ai.exceptions import AgentRunError

from ..ai.errors import GatewayUnavailableError
from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..models.body_fact_intake import BODY_FACT_INTAKE_POLICY
from ..models.body_state_suggestion import body_state_revision, body_state_suggestion_envelope
from ..services.body_state_suggestion_service import BodyStateSuggestionService
from .checkpoint_retention import tag_runtime_message

logger = logging.getLogger(__name__)

SUGGESTION_TOOL_PREFIX = "bodystate-suggestion-v1"

# The runtime's own wording is display-only scaffolding: Go replaces the whole
# question with its fixed confirmation (or an "unverified" notice).
_PLACEHOLDER_QUESTION: dict[str, Any] = {
    "question": "要把你说过的这句话记录下来吗？",
    "answer_type": "single_choice",
    "options": ["记录", "不记录"],
    "allow_custom_input": False,
    "required": True,
}


def suggestion_tool_call_id(run_id: str) -> str:
    return f"{SUGGESTION_TOOL_PREFIX}:{UUID(run_id).hex}"


async def propose_body_state_suggestion(
    state: dict[str, Any],
    *,
    manifest: ConsultationAgentManifest,
    service: BodyStateSuggestionService | None = None,
) -> dict[str, Any]:
    """Propose one suggestion citing this turn's user message, or none.

    The base revision is the BodyState revision Go will hold at bind time:
    the revision of the context Go last sent plus the fact batches this
    stream committed since (``body_state_pending_revisions``). A wrong guess
    only makes Go refuse to bind.
    """
    none: dict[str, Any] = {"body_state_suggestion": None}
    if manifest.intake is None or manifest.intake.policy_revision != BODY_FACT_INTAKE_POLICY:
        return none
    text = str(state.get("latest_user_message") or "").strip()
    message_id = str(state.get("latest_user_message_id") or "")
    body_state = state.get("body_state") or {}
    if (
        not text
        or not message_id
        or state.get("red_flag_result")
        or body_state.get("records_available") is not True
    ):
        return none
    intake = state.get("intake_result") or {}
    episode_statements = tuple(
        statement
        for statement in intake.get("present_symptom_statements") or ()
        if isinstance(statement, str)
    )
    try:
        base_revision = body_state_revision(body_state.get("current_revision")) + int(
            state.get("body_state_pending_revisions") or 0
        )
        output = await (service or BodyStateSuggestionService()).suggest(
            latest_user_message=text,
            episode_statements=episode_statements,
            config=manifest,
        )
        envelope = body_state_suggestion_envelope(
            output,
            text,
            message_id=message_id,
            base_revision=base_revision,
            episode_statements=episode_statements,
        )
    except (AgentRunError, GatewayUnavailableError, ValueError) as error:
        # Bounded retries are exhausted or the context is unusable: this turn
        # has no suggestion. Nothing is guessed and nothing is saved.
        logger.info("No BodyState suggestion this turn: %s", type(error).__name__)
        return none
    return {"body_state_suggestion": envelope}


async def confirm_body_state_suggestion(
    state: dict[str, Any], *, writer: StreamWriter
) -> dict[str, Any]:
    """Pause on the proposed suggestion; the user's answer is Go's to commit."""
    suggestion = state.get("body_state_suggestion")
    if not isinstance(suggestion, dict):
        return {}
    tool_call_id = suggestion_tool_call_id(str(state.get("run_id", "")))
    question = dict(_PLACEHOLDER_QUESTION)
    writer({"type": "tool_call", "id": tool_call_id, "tool": "ask_user", "args": question})
    answer = interrupt(
        {
            "interrupt_type": "ask_user",
            "tool_call_id": tool_call_id,
            "tool_name": "ask_user",
            "question": question,
            "suggestion": suggestion,
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
    runtime_messages = list(state.get("runtime_messages", []))
    runtime_messages.append(
        tag_runtime_message(
            {
                "role": "user",
                "content": "[对记录建议的回答，是否已保存以系统回执为准] "
                + json.dumps(answer, ensure_ascii=False),
            },
            state.get("run_id"),
        )
    )
    return {"body_state_suggestion": None, "runtime_messages": runtime_messages}
