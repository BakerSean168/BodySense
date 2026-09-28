"""Tests for red flag detector."""

import pytest

from src.services.red_flag_detector import (
    RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
    RedFlagDetector,
    get_red_flag_detector,
)

BLOCKER_TEXT = (
    "久坐办公后只有轻微颈肩僵硬和酸胀，活动后会缓解；没有外伤，没有放射痛，"
    "没有麻木，没有无力，也没有头晕。"
)


def test_legacy_literal_revision_preserves_negated_false_positive():
    result = RedFlagDetector().detect([], "没有外伤，没有放射痛，也没有头晕")
    assert {flag.category for flag in result.flags} == {"trauma", "radiating_pain", "neurological"}


def test_negation_aware_revision_suppresses_exact_blocker_phrase():
    result = RedFlagDetector().detect(
        [], BLOCKER_TEXT, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert result.has_red_flags is False


def test_negation_aware_revision_detects_positive_red_flags():
    result = RedFlagDetector().detect(
        [], "外伤后出现放射痛并伴头晕", revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert {flag.category for flag in result.flags} == {"trauma", "radiating_pain", "neurological"}


def test_negation_aware_revision_keeps_mixed_positive_clause():
    result = RedFlagDetector().detect(
        [], "没有外伤，但出现放射痛", revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert {flag.category for flag in result.flags} == {"radiating_pain"}


def test_negation_aware_revision_keeps_current_symptom_after_history():
    result = RedFlagDetector().detect(
        [], "之前没有头晕，现在头晕", revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert {flag.category for flag in result.flags} == {"neurological"}


def test_negation_aware_revision_does_not_cross_source_boundary():
    result = RedFlagDetector().detect(
        [{"additional_notes": "头晕"}],
        "没有",
        revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    )
    assert {flag.category for flag in result.flags} == {"neurological"}


def test_negation_aware_revision_suppresses_negated_notes_independently():
    result = RedFlagDetector().detect(
        [{"additional_notes": "没有头晕"}],
        "没有外伤",
        revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
    )
    assert result.has_red_flags is False


@pytest.mark.parametrize("text", ["不否认头晕", "未否认头晕", "不能说没有头晕"])
def test_negation_aware_revision_keeps_ambiguous_absence_phrases(text: str):
    result = RedFlagDetector().detect(
        [], text, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert {flag.category for flag in result.flags} == {"neurological"}


def test_negation_aware_revision_does_not_suppress_negation_like_positive_phrase():
    result = RedFlagDetector().detect(
        [], "症状无法缓解，休息也不缓解", revision=RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2
    )
    assert {flag.category for flag in result.flags} == {"worsening"}


@pytest.mark.parametrize(
    "text",
    ["没有明显放射痛", "无明显放射痛", "未见明显放射痛", "目前没有出现放射痛", "否认存在放射痛"],
)
def test_negation_bridge_revision_suppresses_only_allowlisted_negative_forms(text: str):
    result = RedFlagDetector().detect(
        [], text, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3
    )
    assert result.has_red_flags is False


@pytest.mark.parametrize("text", ["没有排除放射痛", "不排除放射痛"])
def test_negation_bridge_revision_keeps_exclusions_positive(text: str):
    result = RedFlagDetector().detect(
        [], text, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3
    )
    assert {flag.category for flag in result.flags} == {"radiating_pain"}


def test_negation_bridge_revision_does_not_cross_source_boundary():
    result = RedFlagDetector().detect(
        [{"additional_notes": "放射痛"}],
        "没有明显",
        revision=RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3,
    )
    assert {flag.category for flag in result.flags} == {"radiating_pain"}


@pytest.mark.parametrize(
    "text",
    [
        "无外伤、放射痛、麻木、无力或头晕",
        "阴性症状（无放射痛/麻木/无力/头晕/外伤）",
        "无神经/外伤红旗",
        "无神经压迫或外伤相关红旗信号",
        "且无外伤、放射痛、麻木、无力、头晕等警示症状",
        "无放射痛/麻木/无力/头晕/外伤",
        "无外伤、放射痛",
        "无外伤、神经症状或头晕",
        "无外伤、放射痛、麻木、无力或头晕等红旗信号。",
        "症状由久坐办公触发，活动后改善，持续 2 周且稳定，无外伤、神经症状或头晕。",
    ],
)
def test_negation_list_revision_suppresses_bounded_provider_lists(text: str):
    result = RedFlagDetector().detect(
        [], text, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4
    )
    assert result.has_red_flags is False


@pytest.mark.parametrize(
    "text",
    [
        "无外伤，但出现放射痛",
        "无外伤，后来出现放射痛",
        "无外伤，现在有放射痛",
        "无外伤史，现有放射痛",
        "无外伤、后来出现头晕",
        "无外伤、不排除头晕",
        "无外伤、不能排除头晕",
        "无外伤、头晕明显",
        "无外伤、头晕加重",
        "无外伤、头晕持续",
        "不排除放射痛",
        "未排除放射痛",
        "无法排除外伤",
        "不能排除外伤",
        "不否认头晕",
        "未否认头晕",
        "不能说没有头晕",
        "症状无法缓解，休息也不缓解",
        "之前没有头晕，现在头晕",
        "无外伤，后来出现放射痛",
        "无外伤，但出现放射痛",
        "无外伤,放射痛",
        "无外伤，放射痛",
    ],
)
def test_negation_list_revision_fails_closed_for_positive_or_ambiguous_text(text: str):
    result = RedFlagDetector().detect(
        [], text, revision=RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4
    )
    assert result.has_red_flags is True


def test_negation_list_revision_does_not_cross_source_boundary():
    result = RedFlagDetector().detect(
        [{"additional_notes": "放射痛"}],
        "没有",
        revision=RED_FLAG_DETECTOR_REVISION_NEGATION_LIST_V4,
    )
    assert {flag.category for flag in result.flags} == {"radiating_pain"}


@pytest.mark.parametrize(
    ("revision", "text", "expected"),
    [
        (RED_FLAG_DETECTOR_REVISION_LITERAL_V1, "没有放射痛", True),
        (RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2, "没有放射痛", False),
        (RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3, "没有明显放射痛", False),
        (RED_FLAG_DETECTOR_REVISION_NEGATION_BRIDGE_V3, "不排除放射痛", True),
    ],
)
def test_v1_v2_v3_semantics_remain_unchanged(revision: str, text: str, expected: bool):
    result = RedFlagDetector().detect([], text, revision=revision)
    assert result.has_red_flags is expected


def test_unknown_revision_fails_closed():
    with pytest.raises(ValueError, match="unsupported red-flag detector revision"):
        RedFlagDetector().detect([], "头晕", revision="red-flag-detector-v999")


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
