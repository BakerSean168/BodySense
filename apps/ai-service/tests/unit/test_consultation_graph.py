"""Tests for the LangGraph consultation agent workflow."""

import pytest

from src.services.consultation_graph import (
    ConsultationState,
    _determine_phase,
    _merge_symptoms,
    build_fallback_reply,
    classify_intent,
    decide_phase,
    route_on_action,
    safety_check,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_state(**overrides) -> ConsultationState:
    """Create a minimal ConsultationState for testing."""
    base: ConsultationState = {
        "session_id": "s1",
        "user_id": "u1",
        "user_message": "你好",
        "profile": {},
        "conversation_history": [],
        "rag_results": [],
        "extracted_symptoms": [],
        "red_flag_result": None,
        "intent": "",
        "workflow_action": "",
        "accumulated_text": "",
        "phase": "collecting",
        "llm_available": True,
        "diagnosis_result": None,
        "treatment_result": None,
    }
    base.update(overrides)
    return base


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


# ---------------------------------------------------------------------------
# safety_check node
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_safety_check_no_citation_events():
    """Citations are now emitted by the agent's search_knowledge tool, not safety_check."""
    collected = []
    state = _make_state(
        rag_results=[
            {
                "title": "肩颈问题",
                "summary": "常见问题",
                "body_markdown": "内容",
                "source_title": "来源",
                "category": "posture",
            }
        ]
    )
    result = await safety_check(state, writer=collected.append)

    assert result["red_flag_result"] is None
    # No citation events from safety_check — agent handles RAG via tool
    assert len(collected) == 0


@pytest.mark.asyncio
async def test_safety_check_detects_red_flags():
    collected = []
    state = _make_state(user_message="剧烈疼痛，麻木无力")
    result = await safety_check(state, writer=collected.append)

    assert result["red_flag_result"] is not None
    assert any(e["type"] == "red_flag" for e in collected)


@pytest.mark.asyncio
async def test_safety_check_no_red_flags_for_mild():
    collected = []
    state = _make_state(user_message="肩膀有点酸")
    result = await safety_check(state, writer=collected.append)

    assert result["red_flag_result"] is None
    assert not any(e["type"] == "red_flag" for e in collected)


@pytest.mark.asyncio
async def test_safety_check_citation_body_markdown_truncated():
    """Citation truncation is now handled by _emit_citation_events in generate_response."""
    collected = []
    long_body = "x" * 1000
    state = _make_state(
        rag_results=[{"title": "test", "body_markdown": long_body}]
    )
    await safety_check(state, writer=collected.append)

    # No citations emitted from safety_check anymore
    assert len(collected) == 0


# ---------------------------------------------------------------------------
# classify_intent node
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_classify_intent_supplement_symptom():
    state = _make_state(user_message="我肩膀酸胀")
    result = await classify_intent(state)
    assert result["intent"] == "supplement_symptom"


@pytest.mark.asyncio
async def test_classify_intent_request_analysis():
    state = _make_state(user_message="帮我分析一下")
    result = await classify_intent(state)
    assert result["intent"] == "request_analysis"


@pytest.mark.asyncio
async def test_classify_intent_confirm_diagnosis():
    state = _make_state(user_message="确认，就是这个")
    result = await classify_intent(state)
    assert result["intent"] == "confirm_diagnosis"


@pytest.mark.asyncio
async def test_classify_intent_routes_to_generate_diagnosis():
    state = _make_state(
        user_message="帮我分析",
        extracted_symptoms=[
            {"body_part": "肩部", "symptom_type": "酸胀"}
        ],
    )
    result = await classify_intent(state)
    assert result["intent"] == "request_analysis"
    assert result["workflow_action"] == "generate_diagnosis"


@pytest.mark.asyncio
async def test_classify_intent_routes_to_generate_treatment():
    state = _make_state(
        user_message="确认",
        phase="ready_for_analysis",
    )
    result = await classify_intent(state)
    assert result["workflow_action"] == "generate_treatment"


# ---------------------------------------------------------------------------
# generate_response node
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_generate_response_fallback_no_llm(monkeypatch):
    import src.services.consultation_graph as cg_mod

    def _fail_init(self, *args, **kwargs):
        raise Exception("missing config")

    cg_mod._ai_service_instance = None
    monkeypatch.setattr(cg_mod.AIService, "__init__", _fail_init)

    collected = []
    state = _make_state(user_message="你好")
    from src.services.consultation_graph import generate_response
    result = await generate_response(state, writer=collected.append)

    assert result["llm_available"] is False
    assert len(result["accumulated_text"]) > 0
    text_events = [e for e in collected if isinstance(e, dict) and e.get("type") == "text_delta"]
    assert len(text_events) > 0


@pytest.mark.asyncio
async def test_generate_response_fallback_with_rag(monkeypatch):
    import src.services.consultation_graph as cg_mod

    def _fail_init(self, *args, **kwargs):
        raise Exception("missing config")

    cg_mod._ai_service_instance = None
    monkeypatch.setattr(cg_mod.AIService, "__init__", _fail_init)

    collected = []
    state = _make_state(
        user_message="肘外翻怎么处理",
        rag_results=[{"title": "肘外翻", "summary": "test summary"}],
    )
    from src.services.consultation_graph import generate_response
    result = await generate_response(state, writer=collected.append)

    assert "肘外翻" in result["accumulated_text"]


@pytest.mark.asyncio
async def test_generate_response_streams_text(monkeypatch):
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

    collected = []
    state = _make_state(user_message="你好")
    from src.services.consultation_graph import generate_response
    result = await generate_response(state, writer=collected.append)

    assert result["accumulated_text"] == "你好，请问有什么可以帮助你的？"
    text_events = [e for e in collected if isinstance(e, dict) and e.get("type") == "text_delta"]
    # Text deltas are buffered (~20 char chunks) so short inputs coalesce into fewer events
    combined_text = "".join(e["delta"] for e in text_events)
    assert combined_text == "你好，请问有什么可以帮助你的？"


@pytest.mark.asyncio
async def test_generate_response_processes_tool_calls(monkeypatch):
    events = [
        _text_event("我来帮你分析"),
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

    collected = []
    state = _make_state(user_message="肩膀酸")
    from src.services.consultation_graph import generate_response
    result = await generate_response(state, writer=collected.append)

    assert len(result["extracted_symptoms"]) == 1
    assert result["extracted_symptoms"][0]["body_part"] == "肩部"
    info_events = [
        e for e in collected
        if isinstance(e, dict) and e.get("type") == "extracted_info"
    ]
    assert len(info_events) == 1


# ---------------------------------------------------------------------------
# decide_phase node
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_decide_phase_collecting():
    collected = []
    state = _make_state(extracted_symptoms=[{"body_part": "肩部"}])
    result = await decide_phase(state, writer=collected.append)
    assert result["phase"] == "collecting"


@pytest.mark.asyncio
async def test_decide_phase_ready_for_analysis():
    collected = []
    state = _make_state(
        extracted_symptoms=[{"body_part": "肩部", "symptom_type": "酸胀"}],
        phase="collecting",
    )
    result = await decide_phase(state, writer=collected.append)
    assert result["phase"] == "ready_for_analysis"
    phase_events = [
        e for e in collected
        if isinstance(e, dict) and e.get("type") == "phase_change"
    ]
    assert len(phase_events) == 1


@pytest.mark.asyncio
async def test_decide_phase_no_change_same_phase():
    collected = []
    state = _make_state(
        extracted_symptoms=[{"body_part": "肩部", "symptom_type": "酸胀"}],
        phase="ready_for_analysis",
    )
    result = await decide_phase(state, writer=collected.append)
    assert result["phase"] == "ready_for_analysis"
    phase_events = [
        e for e in collected
        if isinstance(e, dict) and e.get("type") == "phase_change"
    ]
    assert len(phase_events) == 0


# ---------------------------------------------------------------------------
# _merge_symptoms reducer
# ---------------------------------------------------------------------------

def test_merge_symptoms_empty_both():
    assert _merge_symptoms([], []) == []


def test_merge_symptoms_empty_new():
    existing = [{"body_part": "肩部", "symptom_type": "酸胀"}]
    assert _merge_symptoms(existing, []) == existing


def test_merge_symptoms_empty_existing():
    new = [{"body_part": "肩部", "symptom_type": "酸胀"}]
    result = _merge_symptoms([], new)
    assert len(result) == 1
    assert result[0]["body_part"] == "肩部"


def test_merge_symptoms_updates_existing_by_body_part():
    existing = [{"body_part": "肩部", "symptom_type": "酸胀"}]
    new = [{"body_part": "肩部", "duration": "2周"}]
    result = _merge_symptoms(existing, new)
    assert len(result) == 1
    assert result[0]["symptom_type"] == "酸胀"
    assert result[0]["duration"] == "2周"


def test_merge_symptoms_appends_different_body_part():
    existing = [{"body_part": "肩部", "symptom_type": "酸胀"}]
    new = [{"body_part": "腰部", "symptom_type": "疼痛"}]
    result = _merge_symptoms(existing, new)
    assert len(result) == 2


def test_merge_symptoms_skips_empty_body_part():
    existing = [{"body_part": "肩部"}]
    new = [{"body_part": "", "symptom_type": "疼"}]
    result = _merge_symptoms(existing, new)
    assert len(result) == 1


def test_merge_symptoms_allows_clearing_fields():
    existing = [{"body_part": "肩部", "symptom_type": "酸胀", "duration": "2周"}]
    new = [{"body_part": "肩部", "duration": None}]
    result = _merge_symptoms(existing, new)
    assert len(result) == 1
    assert result[0]["symptom_type"] == "酸胀"
    assert result[0]["duration"] is None


# ---------------------------------------------------------------------------
# route_on_action
# ---------------------------------------------------------------------------

def test_route_on_action_maps_to_diagnosis():
    state = _make_state(workflow_action="generate_diagnosis")
    assert route_on_action(state) == "generate_diagnosis"


def test_route_on_action_maps_to_treatment():
    state = _make_state(workflow_action="generate_treatment")
    assert route_on_action(state) == "generate_treatment"


def test_route_on_action_maps_to_emit_done():
    state = _make_state(workflow_action="ask_follow_up")
    assert route_on_action(state) == "emit_done"


def test_route_on_action_empty_defaults_to_emit_done():
    state = _make_state(workflow_action="")
    assert route_on_action(state) == "emit_done"


# ---------------------------------------------------------------------------
# _determine_phase helper
# ---------------------------------------------------------------------------

def test_determine_phase_empty():
    assert _determine_phase([]) == "collecting"


def test_determine_phase_body_part_only():
    assert _determine_phase([{"body_part": "肩部"}]) == "collecting"


def test_determine_phase_with_detail():
    assert _determine_phase([{"body_part": "肩部", "symptom_type": "酸胀"}]) == "ready_for_analysis"


def test_determine_phase_with_duration():
    assert _determine_phase([{"body_part": "肩部", "duration": "2周"}]) == "ready_for_analysis"


# ---------------------------------------------------------------------------
# build_fallback_reply helper
# ---------------------------------------------------------------------------

def test_build_fallback_reply_no_rag():
    result = build_fallback_reply("你好", None)
    assert "没有配置云端大模型" in result


def test_build_fallback_reply_with_rag():
    result = build_fallback_reply("测试", [{"title": "测试标题", "summary": "摘要"}])
    assert "测试标题" in result
    assert "摘要" in result


# ---------------------------------------------------------------------------
# Integration: full graph
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_full_graph_fallback_path(monkeypatch):
    """Full graph with no LLM should emit text and done (citations via agent tool only)."""
    import src.services.consultation_graph as cg_mod

    def _fail_init(self, *args, **kwargs):
        raise Exception("missing config")

    cg_mod._ai_service_instance = None
    monkeypatch.setattr(cg_mod.AIService, "__init__", _fail_init)

    from src.services.consultation_graph import get_consultation_graph

    graph = get_consultation_graph()
    state = _make_state(
        user_message="你好",
        rag_results=[{"title": "知识库条目", "summary": "摘要内容"}],
    )

    collected = []
    async for chunk in graph.astream(state, stream_mode="custom"):
        collected.append(chunk)

    # Should have text_delta events and done event (no citation from safety_check)
    types = [e.get("type") for e in collected if isinstance(e, dict)]
    assert "text_delta" in types
    assert "__done__" in types


@pytest.mark.asyncio
async def test_full_graph_with_tool_calls(monkeypatch):
    """Full graph with tool calls should emit extracted_info events."""
    events = [
        _text_event("分析一下"),
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

    from src.services.consultation_graph import get_consultation_graph

    graph = get_consultation_graph()
    state = _make_state(user_message="肩膀酸")

    collected = []
    async for chunk in graph.astream(state, stream_mode="custom"):
        collected.append(chunk)

    types = [e.get("type") for e in collected if isinstance(e, dict)]
    assert "text_delta" in types
    assert "extracted_info" in types
    assert "__done__" in types


@pytest.mark.asyncio
async def test_full_graph_red_flag_with_llm(monkeypatch):
    """Red flags should be detected even with LLM available."""
    events = [_text_event("注意安全"), _done_event()]
    import src.services.consultation_graph as cg_mod
    cg_mod._ai_service_instance = None
    monkeypatch.setattr(
        "src.services.consultation_graph.AIService",
        lambda: FakeAIService(events),
    )

    from src.services.consultation_graph import get_consultation_graph

    graph = get_consultation_graph()
    state = _make_state(user_message="剧烈疼痛，放射到手臂")

    collected = []
    async for chunk in graph.astream(state, stream_mode="custom"):
        collected.append(chunk)

    types = [e.get("type") for e in collected if isinstance(e, dict)]
    assert "red_flag" in types
    # Red flag should come before text_delta
    red_flag_idx = types.index("red_flag")
    text_idx = types.index("text_delta")
    assert red_flag_idx < text_idx
