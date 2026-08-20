"""Title generation API route."""

import logging
from typing import Any

from fastapi import APIRouter
from pydantic import BaseModel, Field

from ...ai import AiRequest, AIService
from ...ai.gateway import TITLE_ROUTE
from ...ai.types import ChatMessage

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/title", tags=["title"])

_ai_service: AIService | None = None


def _get_ai_service() -> AIService:
    global _ai_service
    if _ai_service is None:
        _ai_service = AIService()
    return _ai_service


class TitleGenerateRequest(BaseModel):
    """Request body for title generation."""

    messages: list[dict[str, Any]] = Field(
        ..., description="Conversation messages to generate a title from"
    )


class TitleGenerateResponse(BaseModel):
    """Response body for title generation."""

    title: str


_TITLE_PROMPT = (
    "你是一个对话标题生成器。根据以下对话内容，生成一个简洁的中文标题来概括这次对话。"
    "要求：\n"
    "- 直接输出标题，不要加任何前缀、引号或解释\n"
    "- 标题应简洁概括对话的核心主题\n"
    "- 如果对话涉及健康咨询，标题应体现主要症状或问题\n"
)


@router.post("/generate", response_model=TitleGenerateResponse)
async def generate_title(request: TitleGenerateRequest):
    """Generate a concise Chinese title for a conversation."""
    ai = _get_ai_service()

    # Build a summary of the conversation for the LLM
    conversation_text = ""
    for msg in request.messages:
        role = msg.get("role", "unknown")
        # Extract text content from message parts
        parts = msg.get("parts", [])
        text_parts = []
        for part in parts:
            if isinstance(part, dict) and part.get("type") == "text":
                text_parts.append(part.get("text", ""))
            elif isinstance(part, str):
                text_parts.append(part)
        content = " ".join(text_parts) if text_parts else msg.get("content", "")
        if content:
            label = "用户" if role == "user" else "助手"
            conversation_text += f"{label}: {content}\n"

    if not conversation_text.strip():
        return TitleGenerateResponse(title="新对话")

    llm_messages = [
        ChatMessage(role="system", content=_TITLE_PROMPT),
        ChatMessage(role="user", content=f"对话内容：\n{conversation_text}"),
    ]

    try:
        req = AiRequest(
            use_case=TITLE_ROUTE,
            messages=llm_messages,
            stream=False,
            temperature=0.3,
            max_tokens=100,
        )
        resp = await ai.generate(req)
        title = resp.text.strip().strip('"').strip("'").strip("《》")
        if not title:
            title = "新对话"
        return TitleGenerateResponse(title=title)
    except Exception:
        logger.exception("Title generation failed")
        return TitleGenerateResponse(title="新对话")
