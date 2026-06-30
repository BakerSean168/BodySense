"""Tests for AIOutputGuard."""

import pytest

from src.services.governance.output_guard import AIOutputGuard
from src.services.governance.types import GovernanceContext, GovernanceStatus


@pytest.fixture
def guard():
    return AIOutputGuard()


def test_validate_text_output_accepted(guard):
    result = guard.validate_text_output("这是一段正常的健康建议文本，包含足够的内容长度。")
    assert result.status == GovernanceStatus.ACCEPTED
    assert len(result.issues) == 0


def test_validate_text_output_degraded_short(guard):
    result = guard.validate_text_output("短")
    assert result.status == GovernanceStatus.DEGRADED
    assert any("too short" in i.message for i in result.issues)


def test_validate_text_output_degraded_empty(guard):
    result = guard.validate_text_output("")
    assert result.status == GovernanceStatus.DEGRADED


def test_validate_structured_output_accepted(guard):
    output = {"diagnoses": [{"name": "圆肩", "confidence": "高"}]}
    result = guard.validate_structured_output(output, required_fields=["diagnoses"])
    assert result.status == GovernanceStatus.ACCEPTED


def test_validate_structured_output_rejected_missing_field(guard):
    output = {"diagnoses": []}
    result = guard.validate_structured_output(output, required_fields=["diagnoses", "treatment_plan"])
    assert result.status == GovernanceStatus.REJECTED
    assert any("Missing required field" in i.message for i in result.issues)


def test_validate_structured_output_no_required_fields(guard):
    output = {"any": "data"}
    result = guard.validate_structured_output(output)
    assert result.status == GovernanceStatus.ACCEPTED


def test_governance_result_to_dict(guard):
    result = guard.validate_text_output("正常文本内容，足够长以通过检查。")
    d = result.to_dict()
    assert "status" in d
    assert "issues" in d
    assert d["status"] == "accepted"


def test_governance_result_to_dict_includes_validated_output(guard):
    result = guard.validate_text_output("正常文本内容，足够长以通过检查。")
    d = result.to_dict()
    assert "validated_output" in d
    assert d["validated_output"] == "正常文本内容，足够长以通过检查。"


def test_validate_treatment_accepted(guard):
    """Treatment plan with exercises grounded in RAG results is accepted."""
    treatment_plan = {
        "correction_exercises": [
            {"name": "臀桥", "sets": 3, "reps": 10},
        ],
    }
    rag_results = [
        {"title": "臀桥训练", "body_markdown": "臀桥是一种常见的臀部激活训练。", "clips": []},
    ]
    context = GovernanceContext(
        output_type="treatment",
        rag_results=rag_results,
    )
    result = guard.validate_treatment(treatment_plan, context)
    # Should not be rejected (may be degraded due to no red flag issues)
    assert result.status in (GovernanceStatus.ACCEPTED, GovernanceStatus.DEGRADED)


def test_validate_treatment_rejected_missing_exercises(guard):
    """Treatment plan without correction_exercises is rejected by schema."""
    treatment_plan = {"other_field": "value"}
    context = GovernanceContext(output_type="treatment")
    result = guard.validate_treatment(treatment_plan, context)
    assert result.status == GovernanceStatus.REJECTED
    assert any("correction_exercises" in i.message for i in result.issues)


def test_validate_treatment_faithfulness_warning(guard):
    """Treatment with ungrounded exercises produces faithfulness warnings."""
    treatment_plan = {
        "correction_exercises": [
            {"name": "完全虚构的动作XYZ", "sets": 3},
        ],
    }
    rag_results = [
        {"title": "臀桥训练", "body_markdown": "臀桥训练内容", "clips": []},
    ]
    context = GovernanceContext(
        output_type="treatment",
        rag_results=rag_results,
    )
    result = guard.validate_treatment(treatment_plan, context)
    faithfulness_issues = [i for i in result.issues if i.policy == "faithfulness"]
    assert len(faithfulness_issues) > 0


def test_validate_treatment_no_rag_skips_faithfulness(guard):
    """Without RAG results, faithfulness check is skipped."""
    treatment_plan = {
        "correction_exercises": [
            {"name": "臀桥", "sets": 3},
        ],
    }
    context = GovernanceContext(output_type="treatment", rag_results=[])
    result = guard.validate_treatment(treatment_plan, context)
    faithfulness_issues = [i for i in result.issues if i.policy == "faithfulness"]
    assert len(faithfulness_issues) == 0
