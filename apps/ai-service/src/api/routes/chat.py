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
                payload = {"type": event.event_type, **event.data}
                yield json.dumps(payload, ensure_ascii=False) + "\n"
        except Exception:
            logger.exception("Error in chat stream")
            yield json.dumps(
                {"type": "error", "message": "Internal error. Please try again."},
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
