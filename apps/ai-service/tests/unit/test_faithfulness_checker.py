"""Tests for RAG faithfulness checking on typed Treatment interventions."""

from src.services.faithfulness_checker import (
    FaithfulnessChecker,
    get_faithfulness_checker,
)


def _treatment(*titles: str) -> dict:
    return {
        "status": "proposed",
        "goal": "测试目标",
        "interventions": [
            {
                "kind": "exercise",
                "title": title,
                "description": "测试动作",
                "prescription": {},
            }
            for title in titles
        ],
    }


def test_exercise_grounded_in_rag_title():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment=_treatment("胸小肌拉伸"),
        rag_results=[{"title": "胸小肌拉伸方法", "summary": "针对圆肩的拉伸", "content": ""}],
    )
    assert result.faithful is True
    assert result.exercises[0].grounded is True
    assert result.exercises[0].source == "胸小肌拉伸方法"
    assert result.exercises[0].confidence == "high"


def test_exercise_grounded_in_rag_content():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment=_treatment("猫牛式"),
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
        treatment=_treatment("深蹲"),
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
        treatment=_treatment("臀桥", "深蹲"),
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


def test_empty_exercises_are_vacuously_faithful():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment=_treatment(),
        rag_results=[],
    )
    assert result.faithful is True
    assert len(result.exercises) == 0


def test_non_exercise_interventions_are_not_faithfulness_checked():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment={
            "status": "proposed",
            "goal": "行为调整",
            "interventions": [{"kind": "habit", "title": "每小时起身"}],
        },
        rag_results=[],
    )
    assert result.faithful is True
    assert result.exercises == []


def test_exercise_grounded_in_clips():
    checker = FaithfulnessChecker()
    result = checker.check_treatment_faithfulness(
        treatment=_treatment("颈部后缩"),
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
        treatment=_treatment("收下巴"),
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
        treatment=_treatment("胸小肌拉伸"),
        rag_results=[{"title": "胸小肌拉伸", "summary": "", "content": ""}],
    )
    data = result.to_dict()
    assert "faithful" in data
    assert "exercises" in data
    assert "ungrounded_exercises" in data
    if data["exercises"]:
        assert "name" in data["exercises"][0]
        assert "grounded" in data["exercises"][0]
        assert "source" in data["exercises"][0]
        assert "confidence" in data["exercises"][0]


def test_singleton_returns_same_instance():
    first = get_faithfulness_checker()
    second = get_faithfulness_checker()
    assert first is second


def test_single_char_exercise_not_grounded():
    checker = get_faithfulness_checker()
    rag = [{"title": "臀桥训练", "body_markdown": "臀桥是很好的臀部训练动作"}]
    result = checker.check_treatment_faithfulness(_treatment("臀"), rag)
    assert result.exercises[0].grounded is False
    assert result.exercises[0].confidence == "low"
