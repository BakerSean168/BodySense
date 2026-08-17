"""Golden case tests for common consultation scenarios."""

from src.services.red_flag_detector import RedFlagDetector


class TestGoldenCases:
    """Golden case tests for common consultation scenarios."""

    def test_shoulder_neck_pain_no_red_flag(self):
        """肩颈酸胀场景：轻度症状不应触发红旗警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[{"body_part": "肩部", "symptom_type": "酸胀", "severity": "轻度"}],
            conversation_text="久坐后肩膀酸胀",
        )
        assert result.has_red_flags is False

    def test_low_back_pain_red_flag_severe(self):
        """腰痛场景：重度症状触发红旗警告。"""
        detector = RedFlagDetector()
        result = detector.detect(
            extracted_info=[{"body_part": "腰部", "symptom_type": "疼痛", "severity": "重度"}],
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
            extracted_info=[{"body_part": "肩部", "symptom_type": "酸胀", "severity": "轻度"}],
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
