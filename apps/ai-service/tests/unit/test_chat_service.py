"""Tests for ChatService.stream_chat — the public API entry point.

Tests verify end-to-end behavior of the chat service: NDJSON event emission,
fallback paths, tool call handling, and phase transitions. Graph internals
(build_messages, _determine_phase) are tested separately in
test_consultation_graph.py.
"""

import pytest

from src.models.consultation import ChatContext
from src.services.chat_service import ChatService


class FakeAIService:
    """Fake AIService that yields deterministic AiStreamEvent objects."""

    def __init__(self, events=None):
        self._events = events or []

    async def generate_stream(self, req):
        for event in self._events:
            yield event


def _text_event(text: str):
    from src.ai.types import AiStreamEvent
    return AiStreamEvent(type="text_delta", text=text)


def _tool_done_event(tool_id: str, name: str, arguments: dict):
    from src.ai.types import AiStreamEvent
    return AiStreamEvent(
        type="tool_call_done",
        tool_call_id=tool_id,
        tool_name=name,
        tool_arguments=arguments,
    )


def _done_event():
    from src.ai.types import AiStreamEvent
    return AiStreamEvent(type="done", finish_reason="stop")


def _events_of_type(events, event_type: str):
    return [event for event in events if event.type == event_type]


@pytest.mark.asyncio
async def test_stream_chat_falls_back_when_llm_missing(monkeypatch):
    """ChatService falls back to local knowledge when no LLM is configured."""
    import src.services.consultation_graph as cg_mod

    def _fail_init(self, *args, **kwargs):
        raise Exception("missing llm config")

    cg_mod._ai_service_instance = None
    monkeypatch.setattr(cg_mod.AIService, "__init__", _fail_init)

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

    message_events = [e for e in events if e.channel == "message"]
    assert message_events

    text_events = _events_of_type(message_events, "message.text.delta")
    assert any("肘外翻怎么处理" in event.payload["delta"] for event in text_events)
    assert any("动作演示" in event.payload["delta"] for event in text_events)

    done_event = events[-1]
    assert done_event.type == "stream.done"
    assert "本地 curated 知识库" in done_event.payload["full_text"]
    assert done_event.payload["phase"] == "collecting"


@pytest.mark.asyncio
async def test_stream_chat_happy_path_with_text(monkeypatch):
    """stream_chat with LLM: text chunks are accumulated and emitted."""
    events = [
        _text_event("你好，"),
        _text_event("请问有什么"),
        _text_event("可以帮助你的？"),
        _done_event(),
    ]
    import src.services.consultation_graph as cg_mod
    cg_mod._ai_service_instance = None
    monkeypatch.setattr(
        "src.services.consultation_graph.AIService",
        lambda: FakeAIService(events),
    )

    service = ChatService()
    context = ChatContext(session_id="s1", user_id="u1")
    result_events = []
    async for event in service.stream_chat(context, "你好"):
        result_events.append(event)

    text_events = _events_of_type(result_events, "message.text.delta")
    # Text deltas are buffered (~20 char chunks) so short inputs coalesce into one event
    combined_text = "".join(e.payload["delta"] for e in text_events)
    assert combined_text == "你好，请问有什么可以帮助你的？"

    done_event = result_events[-1]
    assert done_event.type == "stream.done"
    assert done_event.payload["full_text"] == "你好，请问有什么可以帮助你的？"
    assert done_event.payload["session_id"] == "s1"


@pytest.mark.asyncio
async def test_stream_chat_with_tool_call(monkeypatch):
    """stream_chat with tool call: extracted_info event is emitted."""
    events = [
        _text_event("我来帮你分析一下"),
        _tool_done_event(
            "tc1", "extract_symptom_info",
            {"body_part": "肩部", "symptom_type": "酸胀"},
        ),
        _done_event(),
    ]
    import src.services.consultation_graph as cg_mod
    cg_mod._ai_service_instance = None
    monkeypatch.setattr(
        "src.services.consultation_graph.AIService",
        lambda: FakeAIService(events),
    )

    service = ChatService()
    context = ChatContext(session_id="s1", user_id="u1")
    result_events = []
    async for event in service.stream_chat(context, "肩膀酸"):
        result_events.append(event)

    info_events = _events_of_type(result_events, "state.extracted_info.upsert")
    assert len(info_events) == 1
    assert info_events[0].payload["info"]["body_part"] == "肩部"
    assert info_events[0].payload["info"]["symptom_type"] == "酸胀"


@pytest.mark.asyncio
async def test_stream_chat_emits_phase_changed(monkeypatch):
    """stream_chat emits phase_change when symptom detail is present."""
    events = [
        _text_event("了解"),
        _done_event(),
    ]
    monkeypatch.setattr(
        "src.services.consultation_graph.AIService",
        lambda: FakeAIService(events),
    )
    # Reset the singleton so it picks up the patched AIService
    import src.services.consultation_graph as cg_mod
    cg_mod._ai_service_instance = None

    service = ChatService()
    context = ChatContext(session_id="s1", user_id="u1")
    context.extracted_info.add_symptom({"body_part": "肩部", "symptom_type": "酸胀"})
    result_events = []
    async for event in service.stream_chat(context, "继续"):
        result_events.append(event)

    phase_events = _events_of_type(result_events, "state.phase.changed")
    assert len(phase_events) == 1
    assert phase_events[0].payload["to"] == "ready_for_analysis"


@pytest.mark.asyncio
async def test_stream_chat_emits_red_flag_before_text(monkeypatch):
    """stream_chat emits red_flag event BEFORE text when red flags detected."""
    events = [
        _text_event("注意安全"),
        _done_event(),
    ]
    import src.services.consultation_graph as cg_mod
    cg_mod._ai_service_instance = None
    monkeypatch.setattr(
        "src.services.consultation_graph.AIService",
        lambda: FakeAIService(events),
    )

    service = ChatService()
    context = ChatContext(session_id="s1", user_id="u1")
    result_events = []
    async for event in service.stream_chat(context, "剧烈疼痛"):
        result_events.append(event)

    # red_flag should be the FIRST message event
    assert result_events[0].type == "safety.red_flag.detected"
    assert result_events[0].payload["has_red_flags"] is True

    # text events should follow
    text_events = _events_of_type(result_events, "message.text.delta")
    assert len(text_events) > 0


@pytest.mark.asyncio
async def test_stream_chat_fallback_no_rag(monkeypatch):
    """stream_chat fallback with no RAG results returns generic message."""
    import src.services.consultation_graph as cg_mod

    def _fail_init(self, *args, **kwargs):
        raise Exception("missing llm config")

    cg_mod._ai_service_instance = None
    monkeypatch.setattr(cg_mod.AIService, "__init__", _fail_init)

    service = ChatService()
    context = ChatContext(session_id="s1", user_id="u1")
    result_events = []
    async for event in service.stream_chat(context, "你好", rag_results=None):
        result_events.append(event)

    done_event = result_events[-1]
    assert "没有配置云端大模型" in done_event.payload["full_text"]
    assert "知识库里暂时没有检索到" in done_event.payload["full_text"]
