"""Chat API routes for consultation sessions."""

import json
from typing import Any

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from ...models.consultation import ChatContext, ExtractedInfo
from ...services.chat_service import ChatService

router = APIRouter(prefix="/api/chat", tags=["chat"])

# In-memory session cache (in production, use Redis)
_sessions: dict[str, ChatContext] = {}


class ChatRequest(BaseModel):
    """Request body for chat message."""

    session_id: str
    user_id: str
    content: str
    profile: dict[str, Any] = Field(default_factory=dict)
    messages: list[dict[str, Any]] = Field(default_factory=list)
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    rag_results: list[dict[str, Any]] = Field(default_factory=list)


class SessionInfo(BaseModel):
    """Session information response."""

    session_id: str
    extracted_info: list[dict[str, Any]]


@router.post("/stream")
async def chat_stream(request: ChatRequest):
    """
    Stream a chat response via SSE.

    The request includes the full session context (messages, profile, extracted info)
    and the new user message. The response is streamed as SSE events.
    """
    # Build context from request
    context = ChatContext(
        session_id=request.session_id,
        user_id=request.user_id,
        profile=request.profile,
        extracted_info=ExtractedInfo.from_dict(request.extracted_info),
        messages=request.messages,
    )

    chat_service = ChatService()

    async def event_generator():
        async for event in chat_service.stream_chat(
            context=context,
            user_message=request.content,
            rag_results=request.rag_results,
        ):
            event_data = json.dumps(event.data, ensure_ascii=False)
            yield f"event: {event.event_type}\ndata: {event_data}\n\n"

    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )
