"""Tests for agent workflow."""

from src.models.consultation import ChatContext, ExtractedInfo, SymptomInfo
from src.services.agent_workflow import (
    AgentAction,
    ConsultationAgentWorkflow,
    ConsultationIntent,
    get_agent_workflow,
)


def _make_context(symptoms=None, phase="collecting") -> ChatContext:
    """Helper to create a test context."""
    ctx = ChatContext(session_id="s1", user_id="u1")
    if symptoms:
        for s in symptoms:
            ctx.extracted_info.symptoms.append(SymptomInfo(**s))
    ctx.phase = phase
    return ctx


# --- Intent classification tests ---


def test_classify_supplement_symptom():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("我肩膀酸胀，久坐后更明显", ctx)
    assert intent == ConsultationIntent.SUPPLEMENT_SYMPTOM


def test_classify_request_analysis():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("帮我分析一下是什么问题", ctx)
    assert intent == ConsultationIntent.REQUEST_ANALYSIS


def test_classify_request_analysis_check_keyword():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("我想做个检查", ctx)
    assert intent == ConsultationIntent.REQUEST_ANALYSIS


def test_classify_confirm_diagnosis():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context(phase="ready_for_analysis")
    intent = wf.classify_intent("确认，就是第一个", ctx)
    assert intent == ConsultationIntent.CONFIRM_DIAGNOSIS


def test_classify_request_treatment():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("怎么改善呢", ctx)
    assert intent == ConsultationIntent.REQUEST_TREATMENT


def test_classify_clarification():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("什么意思，能详细说说吗", ctx)
    assert intent == ConsultationIntent.CLARIFICATION


def test_classify_general_question():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    intent = wf.classify_intent("你好", ctx)
    assert intent == ConsultationIntent.GENERAL_QUESTION


def test_classify_context_refinement_confirm():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context(phase="ready_for_analysis")
    intent = wf.classify_intent("好", ctx)
    assert intent == ConsultationIntent.CONFIRM_DIAGNOSIS


# --- Merge extracted info tests ---


def test_merge_new_body_part():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    added = wf.merge_extracted_info(info, {"body_part": "肩部", "symptom_type": "酸胀"})
    assert added is True
    assert len(info.symptoms) == 1
    assert info.symptoms[0].body_part == "肩部"


def test_merge_update_existing():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    info.symptoms.append(SymptomInfo(body_part="肩部", symptom_type="酸胀"))
    added = wf.merge_extracted_info(
        info, {"body_part": "肩部", "duration": "2周"}
    )
    assert added is True
    assert info.symptoms[0].duration == "2周"
    assert info.symptoms[0].symptom_type == "酸胀"


def test_merge_no_change():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    info.symptoms.append(SymptomInfo(body_part="肩部", symptom_type="酸胀"))
    added = wf.merge_extracted_info(
        info, {"body_part": "肩部", "symptom_type": "酸胀"}
    )
    assert added is False


def test_merge_empty_body_part():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    added = wf.merge_extracted_info(info, {"body_part": "", "symptom_type": "酸胀"})
    assert added is False


# --- Should analyze tests ---


def test_should_analyze_with_detail():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    info.symptoms.append(SymptomInfo(body_part="肩部", symptom_type="酸胀"))
    assert wf.should_analyze(info) is True


def test_should_analyze_without_detail():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    info.symptoms.append(SymptomInfo(body_part="肩部"))
    assert wf.should_analyze(info) is False


def test_should_analyze_empty():
    wf = ConsultationAgentWorkflow()
    info = ExtractedInfo()
    assert wf.should_analyze(info) is False


# --- Should generate treatment tests ---


def test_should_generate_treatment_confirmed():
    wf = ConsultationAgentWorkflow()
    assert wf.should_generate_treatment("diagnosis_confirmed", True) is True


def test_should_generate_treatment_wrong_phase():
    wf = ConsultationAgentWorkflow()
    assert wf.should_generate_treatment("collecting", True) is False


def test_should_generate_treatment_not_confirmed():
    wf = ConsultationAgentWorkflow()
    assert wf.should_generate_treatment("diagnosis_confirmed", False) is False


# --- Decide next action tests ---


def test_decide_action_supplement_symptom():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    decision = wf.decide_next_action(
        ConsultationIntent.SUPPLEMENT_SYMPTOM, ctx, "我肩膀酸胀，久坐后更明显"
    )
    assert decision.action == AgentAction.ASK_FOLLOW_UP
    assert decision.confidence > 0.5


def test_decide_action_analysis_ready():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context(
        symptoms=[{"body_part": "肩部", "symptom_type": "酸胀"}]
    )
    decision = wf.decide_next_action(
        ConsultationIntent.REQUEST_ANALYSIS, ctx, "帮我分析一下是什么问题"
    )
    assert decision.action == AgentAction.GENERATE_DIAGNOSIS


def test_decide_action_analysis_not_ready():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    decision = wf.decide_next_action(
        ConsultationIntent.REQUEST_ANALYSIS, ctx, "帮我看看是不是头前移"
    )
    assert decision.action == AgentAction.PROVIDE_INFO


def test_decide_action_confirm_diagnosis():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    decision = wf.decide_next_action(
        ConsultationIntent.CONFIRM_DIAGNOSIS, ctx, "确认，就是这个"
    )
    assert decision.action == AgentAction.GENERATE_TREATMENT


def test_decide_action_treatment_with_diagnosis():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context(phase="diagnosis_confirmed")
    decision = wf.decide_next_action(
        ConsultationIntent.REQUEST_TREATMENT, ctx, "怎么改善呢"
    )
    assert decision.action == AgentAction.GENERATE_TREATMENT


def test_decide_action_treatment_without_diagnosis():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context(phase="collecting")
    decision = wf.decide_next_action(
        ConsultationIntent.REQUEST_TREATMENT, ctx, "怎么改善呢"
    )
    assert decision.action == AgentAction.ASK_FOLLOW_UP


def test_decide_action_clarification():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    decision = wf.decide_next_action(
        ConsultationIntent.CLARIFICATION, ctx, "什么意思，能详细说说吗"
    )
    assert decision.action == AgentAction.PROVIDE_INFO


def test_decide_action_posture_observation_prefers_provide_info():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    decision = wf.decide_next_action(
        ConsultationIntent.GENERAL_QUESTION,
        ctx,
        "我感觉自己有点头前移",
    )
    assert decision.action == AgentAction.PROVIDE_INFO
    assert "posture observation" in decision.reasoning


def test_should_interrupt_for_follow_up_on_critical_signal():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    assert wf.should_interrupt_for_follow_up("手臂麻木无力，还往下放射", ctx) is True


def test_should_not_interrupt_for_initial_posture_observation():
    wf = ConsultationAgentWorkflow()
    ctx = _make_context()
    assert wf.should_interrupt_for_follow_up("我感觉自己有点头前移", ctx) is False


# --- Singleton test ---


def test_singleton_returns_same_instance():
    w1 = get_agent_workflow()
    w2 = get_agent_workflow()
    assert w1 is w2
