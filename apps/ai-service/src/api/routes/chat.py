"""Chat API routes for consultation sessions."""

import json
import logging
from typing import Any

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from ...models.consultation import ChatContext, ExtractedInfo
from ...services.chat_service import ChatService

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/chat", tags=["chat"])

# Module-level singleton — ChatService is stateless
_chat_service = ChatService()


class ChatRequest(BaseModel):
    """Request body for chat message."""

    session_id: str
    user_id: str
    content: str
    use_case: str = "consultation.reply"
    profile: dict[str, Any] = Field(default_factory=dict)
    messages: list[dict[str, Any]] = Field(default_factory=list)
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    rag_results: list[dict[str, Any]] = Field(default_factory=list)
    phase: str = "collecting"


class ResumeRequest(BaseModel):
    """Request body for resuming an interrupted ask_user interaction."""

    session_id: str
    user_id: str
    interaction_id: str
    answer: dict[str, Any]
    profile: dict[str, Any] = Field(default_factory=dict)
    messages: list[dict[str, Any]] = Field(default_factory=list)
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    rag_results: list[dict[str, Any]] = Field(default_factory=list)
    phase: str = "collecting"


@router.post("/stream")
async def chat_stream(request: ChatRequest):
    """
    Stream a chat response as NDJSON (one JSON object per line).

    The request includes the full session context (messages, profile, extracted info)
    and the new user message. Each line of the response is a self-contained JSON object.
    """
    # Build context from request
    context = ChatContext(
        session_id=request.session_id,
        user_id=request.user_id,
        profile=request.profile,
        extracted_info=ExtractedInfo.from_dict(request.extracted_info),
        messages=request.messages,
        phase=request.phase,
    )

    async def ndjson_generator():
        try:
            async for event in _chat_service.stream_chat(
                context=context,
                user_message=request.content,
                rag_results=request.rag_results,
            ):
                payload = event.model_dump(exclude_none=True)
                yield json.dumps(payload, ensure_ascii=False) + "\n"
        except Exception:
            logger.exception("Error in chat stream")
            yield json.dumps(
                {
                    "version": 1,
                    "seq": 1,
                    "channel": "stream",
                    "type": "stream.error",
                    "ids": {},
                    "payload": {"message": "Internal error. Please try again."},
                },
                ensure_ascii=False,
            ) + "\n"

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


@router.post("/resume")
async def chat_resume(request: ResumeRequest):
    """Resume an interrupted ask_user interaction.

    Re-invokes the consultation graph with the user's answer injected
    as a tool result message, allowing the LLM to continue from where
    it was interrupted.
    """
    context = ChatContext(
        session_id=request.session_id,
        user_id=request.user_id,
        profile=request.profile,
        extracted_info=ExtractedInfo.from_dict(request.extracted_info),
        messages=request.messages,
        phase=request.phase,
    )

    # Inject the user's answer as a tool result into conversation history
    # so the LLM sees the answer and continues naturally
    answer_text = request.answer.get("text", "")
    if not answer_text:
        # For choice types, join selected options
        answer_text = ", ".join(request.answer.get("selected", []))

    resume_context_messages = list(request.messages)
    resume_context_messages.append({
        "role": "tool",
        "tool_call_id": request.interaction_id,
        "content": f"用户回答：{answer_text}",
    })

    async def ndjson_generator():
        try:
            async for event in _chat_service.stream_chat(
                context=ChatContext(
                    session_id=context.session_id,
                    user_id=context.user_id,
                    profile=context.profile,
                    extracted_info=context.extracted_info,
                    messages=resume_context_messages,
                    phase=context.phase,
                ),
                user_message=answer_text,
                rag_results=request.rag_results,
            ):
                payload = event.model_dump(exclude_none=True)
                yield json.dumps(payload, ensure_ascii=False) + "\n"
        except Exception:
            logger.exception("Error in chat resume")
            yield json.dumps(
                {
                    "version": 1,
                    "seq": 1,
                    "channel": "stream",
                    "type": "stream.error",
                    "ids": {},
                    "payload": {"message": "Internal error during resume."},
                },
                ensure_ascii=False,
            ) + "\n"

    return StreamingResponse(
        ndjson_generator(),
        media_type="application/x-ndjson",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
