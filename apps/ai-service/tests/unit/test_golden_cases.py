"""Golden case tests for common consultation scenarios."""

from src.models.consultation import ChatContext, ExtractedInfo, SymptomInfo
from src.services.agent_workflow import ConsultationAgentWorkflow, ConsultationIntent
from src.services.red_flag_detector import RedFlagDetector


def _make_context(symptoms=None, phase="collecting") -> ChatContext:
    """Helper to create a test context."""
    ctx = ChatContext(session_id="s1", user_id="u1")
    if symptoms:
        for s in symptoms:
            ctx.extracted_info.symptoms.append(SymptomInfo(**s))
    ctx.phase = phase
    return ctx


class TestGoldenCases:
    """Golden case tests for common consultation scenarios."""

    def test_shoulder_neck_pain_symptom_extraction(self):
        """肩颈酸胀场景：症状信息提取正确。"""
        ctx = _make_context(
            symptoms=[
                {
                    "body_part": "肩部",
                    "symptom_type": "酸胀",
                    "duration": "2周",
                    "trigger": "久坐后",
                    "severity": "轻度",
                }
            ]
        )
        assert len(ctx.extracted_info.symptoms) == 1
        symptom = ctx.extracted_info.symptoms[0]
        assert symptom.body_part == "肩部"
        assert symptom.symptom_type == "酸胀"
        assert symptom.duration == "2周"

    def test_shoulder_neck_pain_ready_for_analysis(self):
        """肩颈酸胀场景：信息足够时可进入分析阶段。"""
        wf = ConsultationAgentWorkflow()
        ctx = _make_context(
            symptoms=[
                {
                    "body_part": "肩部",
                    "symptom_type": "酸胀",
                    "duration": "2周",
                    "trigger": "久坐后",
                }
            ]
        )
        assert wf.should_analyze(ctx.extracted_info) is True

    def test_shoulder_neck_pain_no_red_flag(self):
        """肩颈酸胀场景：轻度症状不应触发红旗警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[
                {"body_part": "肩部", "symptom_type": "酸胀", "severity": "轻度"}
            ],
            conversation_text="久坐后肩膀酸胀",
        )
        assert result.has_red_flags is False

    def test_forward_head_posture_intent(self):
        """头前伸场景：用户描述症状时应识别为补充症状意图。"""
        wf = ConsultationAgentWorkflow()
        ctx = _make_context()
        intent = wf.classify_intent(
            "我发现自己头颈前伸，经常颈肩酸胀",
            ctx,
        )
        assert intent == ConsultationIntent.SUPPLEMENT_SYMPTOM

    def test_forward_head_posture_analysis_ready(self):
        """头前伸场景：有足够信息时可进入分析。"""
        wf = ConsultationAgentWorkflow()
        ctx = _make_context(
            symptoms=[
                {
                    "body_part": "颈椎",
                    "symptom_type": "酸胀",
                    "trigger": "久坐低头后",
                    "severity": "中度",
                },
                {
                    "body_part": "肩部",
                    "symptom_type": "僵硬",
                },
            ]
        )
        assert wf.should_analyze(ctx.extracted_info) is True

    def test_cubitus_valgus_symptom_merge(self):
        """肘外翻场景：同一部位信息可合并更新。"""
        wf = ConsultationAgentWorkflow()
        info = ExtractedInfo()
        info.symptoms.append(
            SymptomInfo(body_part="肘部", symptom_type="外翻角度偏大")
        )
        updated = wf.merge_extracted_info(
            info, {"body_part": "肘部", "duration": "自幼", "severity": "轻度"}
        )
        assert updated is True
        assert info.symptoms[0].duration == "自幼"
        assert info.symptoms[0].severity == "轻度"

    def test_low_back_pain_with_trigger(self):
        """腰痛场景：触发场景信息正确提取。"""
        ctx = _make_context(
            symptoms=[
                {
                    "body_part": "腰部",
                    "symptom_type": "钝痛",
                    "duration": "1个月",
                    "trigger": "久坐后",
                    "relief": "起身活动后缓解",
                    "severity": "中度",
                }
            ]
        )
        symptom = ctx.extracted_info.symptoms[0]
        assert symptom.trigger == "久坐后"
        assert symptom.relief == "起身活动后缓解"

    def test_low_back_pain_red_flag_severe(self):
        """腰痛场景：重度症状触发红旗警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[
                {"body_part": "腰部", "symptom_type": "疼痛", "severity": "重度"}
            ],
            conversation_text="",
        )
        assert result.has_red_flags is True
        assert any(f.category == "severe_symptom" for f in result.flags)


class TestRedFlagGoldenCases:
    """Golden cases for red flag detection."""

    def test_red_flag_severe_pain(self):
        """红旗症状：严重疼痛应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="肩膀剧烈疼痛，疼得睡不着",
        )
        assert result.has_red_flags is True
        assert any(f.category == "severe_pain" for f in result.flags)

    def test_red_flag_numbness(self):
        """红旗症状：麻木无力应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="手指麻木无力，握不住东西",
        )
        assert result.has_red_flags is True
        assert any(f.category == "numbness" for f in result.flags)

    def test_red_flag_trauma(self):
        """红旗症状：外伤应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="摔倒后腰疼得厉害",
        )
        assert result.has_red_flags is True
        assert any(f.category == "trauma" for f in result.flags)

    def test_red_flag_worsening(self):
        """红旗症状：症状持续加重应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="疼痛越来越严重，休息也无法缓解",
        )
        assert result.has_red_flags is True
        assert any(f.category == "worsening" for f in result.flags)

    def test_red_flag_radiating_pain(self):
        """红旗症状：放射痛应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="疼痛放射到手臂",
        )
        assert result.has_red_flags is True
        assert any(f.category == "radiating_pain" for f in result.flags)

    def test_no_red_flag_mild_symptoms(self):
        """非红旗：轻度酸胀不应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[
                {"body_part": "肩部", "symptom_type": "酸胀", "severity": "轻度"}
            ],
            conversation_text="久坐后肩膀有点酸",
        )
        assert result.has_red_flags is False

    def test_no_red_flag_general_consultation(self):
        """非红旗：一般咨询不应触发警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[],
            conversation_text="你好，我想咨询一下体态问题",
        )
        assert result.has_red_flags is False
