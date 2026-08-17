"""Tests for red flag detector."""

from src.services.red_flag_detector import RedFlagDetector, get_red_flag_detector


def test_detect_severe_pain_in_conversation():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="我最近肩膀剧烈疼痛，疼得睡不着",
    )
    assert result.has_red_flags is True
    flag = next(f for f in result.flags if f.category == "severe_pain")
    assert flag.message  # message should be non-empty
    assert flag.source == "conversation"


def test_detect_radiating_pain():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="疼痛放射到手臂",
    )
    assert result.has_red_flags is True
    assert any(f.category == "radiating_pain" for f in result.flags)


def test_detect_numbness():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="手指麻木无力",
    )
    assert result.has_red_flags is True
    assert any(f.category == "numbness" for f in result.flags)


def test_detect_trauma():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="上周摔倒后腰一直疼",
    )
    assert result.has_red_flags is True
    assert any(f.category == "trauma" for f in result.flags)


def test_detect_worsening_symptoms():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="症状越来越严重，无法缓解",
    )
    assert result.has_red_flags is True
    assert any(f.category == "worsening" for f in result.flags)


def test_detect_severity_in_extracted_info():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[{"body_part": "腰部", "severity": "重度", "symptom_type": "疼痛"}],
        conversation_text="",
    )
    assert result.has_red_flags is True
    flag = next(f for f in result.flags if f.category == "severe_symptom")
    assert flag.message  # message should be non-empty
    assert flag.source == "extracted_info"


def test_no_red_flags_for_mild_symptoms():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[{"body_part": "肩部", "severity": "轻度", "symptom_type": "酸胀"}],
        conversation_text="久坐后肩膀有点酸",
    )
    assert result.has_red_flags is False
    assert len(result.flags) == 0


def test_no_red_flags_for_general_question():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="你好，我想咨询一下体态问题",
    )
    assert result.has_red_flags is False


def test_multiple_red_flags():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="剧烈疼痛，手指麻木无力，症状持续加重",
    )
    assert result.has_red_flags is True
    assert len(result.flags) >= 3


def test_deduplicate_same_category():
    detector = RedFlagDetector()
    result = detector.detect(
        extracted_info=[],
        conversation_text="剧烈疼痛，严重疼痛，疼痛难忍",
    )
    severe_pain_flags = [f for f in result.flags if f.category == "severe_pain"]
    assert len(severe_pain_flags) == 1


def test_is_red_flag_convenience_method():
    detector = RedFlagDetector()
    assert detector.is_red_flag([], "剧烈疼痛") is True
    assert detector.is_red_flag([], "肩膀有点酸") is False


def test_to_dict_format():
    detector = RedFlagDetector()
    result = detector.detect([], "剧烈疼痛")
    d = result.to_dict()
    assert d["has_red_flags"] is True
    assert isinstance(d["flags"], list)
    assert len(d["flags"]) >= 1
    flag = d["flags"][0]
    assert "category" in flag
    assert "message" in flag
    assert "matched_text" in flag
    assert "source" in flag


def test_singleton_returns_same_instance():
    d1 = get_red_flag_detector()
    d2 = get_red_flag_detector()
    assert d1 is d2
