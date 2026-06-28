"""Tests for RAG faithfulness checker."""

from src.services.faithfulness_checker import (
    FaithfulnessChecker,
    get_faithfulness_checker,
)


def test_exercise_grounded_in_rag_title():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "胸小肌拉伸", "description": "靠门框进行温和拉伸。"}
            ]
        },
        rag_results=[
            {"title": "胸小肌拉伸方法", "summary": "针对圆肩的拉伸", "content": ""}
        ],
    )
    assert result.faithful is True
    assert result.exercises[0].grounded is True
    assert result.exercises[0].source == "胸小肌拉伸方法"
    assert result.exercises[0].confidence == "high"


def test_exercise_grounded_in_rag_content():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "猫牛式", "description": "四点跪位脊柱屈伸。"}
            ]
        },
        rag_results=[
            {
                "title": "腰痛改善动作",
                "summary": "常见改善动作",
                "content": "推荐猫牛式和臀桥训练。",
            }
        ],
    )
    assert result.faithful is True
    assert result.exercises[0].grounded is True
    assert result.exercises[0].confidence == "medium"


def test_exercise_not_grounded():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "深蹲", "description": "负重深蹲训练。"}
            ]
        },
        rag_results=[
            {
                "title": "肩颈改善",
                "summary": "针对头前伸",
                "content": "推荐颈部后缩和肩胛骨回缩。",
            }
        ],
    )
    assert result.faithful is False
    assert result.exercises[0].grounded is False
    assert "深蹲" in result.ungrounded_exercises


def test_mixed_grounded_and_ungrounded():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "臀桥", "description": "仰卧臀桥。"},
                {"name": "深蹲", "description": "负重深蹲。"},
            ]
        },
        rag_results=[
            {
                "title": "腰痛改善",
                "summary": "改善方案",
                "content": "臀桥是核心稳定的基础动作。",
            }
        ],
    )
    assert result.faithful is False
    assert result.exercises[0].grounded is True
    assert result.exercises[1].grounded is False
    assert len(result.ungrounded_exercises) == 1


def test_empty_exercises():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={"correction_exercises": []},
        rag_results=[],
    )
    assert result.faithful is False
    assert len(result.exercises) == 0


def test_no_exercises_key():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={},
        rag_results=[],
    )
    assert result.faithful is False


def test_exercise_grounded_in_clips():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "颈部后缩", "description": "收下巴训练。"}
            ]
        },
        rag_results=[
            {
                "title": "头前伸改善",
                "summary": "改善头前伸",
                "content": "通用内容",
                "clips": [{"title": "颈部后缩训练演示"}],
            }
        ],
    )
    assert result.faithful is True
    assert result.exercises[0].confidence == "high"


def test_alias_matching():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "收下巴", "description": "颈椎后缩训练。"}
            ]
        },
        rag_results=[
            {
                "title": "头前伸改善",
                "summary": "改善头前伸",
                "content": "推荐颈部后缩训练。",
            }
        ],
    )
    assert result.faithful is True
    assert result.exercises[0].grounded is True


def test_to_dict_format():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment_plan={
            "correction_exercises": [
                {"name": "胸小肌拉伸", "description": "拉伸。"}
            ]
        },
        rag_results=[
            {"title": "胸小肌拉伸", "summary": "", "content": ""}
        ],
    )
    d = result.to_dict()
    assert "faithful" in d
    assert "exercises" in d
    assert "ungrounded_exercises" in d
    if d["exercises"]:
        assert "name" in d["exercises"][0]
        assert "grounded" in d["exercises"][0]
        assert "source" in d["exercises"][0]
        assert "confidence" in d["exercises"][0]


def test_singleton_returns_same_instance():
    c1 = get_faithfulness_checker()
    c2 = get_faithfulness_checker()
    assert c1 is c2


def test_single_char_exercise_not_grounded():
    """Single-character exercise names should not match (guard against false positives)."""
    checker = get_faithfulness_checker()
    plan = {"correction_exercises": [{"name": "臀"}]}
    rag = [{"title": "臀桥训练", "body_markdown": "臀桥是很好的臀部训练动作"}]
    result = checker.check_treatment_faithfulness(plan, rag)
    # "臀" is only 1 char after normalization — should be rejected by length guard
    assert result.exercises[0].grounded is False
    assert result.exercises[0].confidence == "low"
