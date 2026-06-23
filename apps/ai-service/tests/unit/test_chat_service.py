import pytest

from src.models.consultation import ChatContext
from src.services.chat_service import ChatService


@pytest.mark.asyncio
async def test_stream_chat_falls_back_to_local_knowledge_when_llm_missing(monkeypatch):
    monkeypatch.setattr(
        "src.services.chat_service.get_llm_provider",
        lambda: (_ for _ in ()).throw(ValueError("missing llm key")),
    )

    service = ChatService()
    context = ChatContext(session_id="session-1", user_id="user-1")
    events = []

    async for event in service.stream_chat(
        context=context,
        user_message="肘外翻怎么处理",
        rag_results=[
            {
                "title": "肘外翻怎么处理",
                "summary": "可先做关节松动，再做减压，最后增加外部支撑。",
                "body_markdown": "## 推荐顺序\n1. 关节松动\n2. 减压\n3. 支撑",
                "clips": [{"title": "肘外翻基础观察演示"}],
            }
        ],
    ):
        events.append(event)

    message_events = [event for event in events if event.event_type == "message"]
    assert message_events
    assert any("肘外翻怎么处理" in event.data["content"] for event in message_events)
    assert any("动作演示" in event.data["content"] for event in message_events)

    done_event = events[-1]
    assert done_event.event_type == "done"
    assert "本地 curated 知识库" in done_event.data["full_text"]
