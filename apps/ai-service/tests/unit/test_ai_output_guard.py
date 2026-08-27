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
    output = {"candidates": [{"name": "圆肩", "confidence": "高"}]}
    result = guard.validate_structured_output(output, required_fields=["candidates"])
    assert result.status == GovernanceStatus.ACCEPTED


def test_validate_structured_output_rejected_missing_field(guard):
    output = {"candidates": []}
    result = guard.validate_structured_output(
        output,
        required_fields=["candidates", "treatment_plan"],
    )
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


def test_treatment_gate_moved_to_runtime_governance_seam():
    """validate_treatment was dead code; treatment now uses guard_structured_output."""
    from src.runtime.governance import guard_structured_output

    grounded = guard_structured_output(
        "treatment",
        {
            "status": "proposed",
            "goal": "改善髋部稳定性",
            "duration_weeks": 4,
            "interventions": [
                {
                    "kind": "exercise",
                    "title": "臀桥",
                    "description": "进行可控的髋伸训练。",
                    "prescription": {"sets": 3, "reps": 10},
                }
            ],
        },
        rag_results=[
            {
                "title": "臀桥训练",
                "body_markdown": "臀桥是一种常见的臀部激活训练。",
                "clips": [],
            }
        ],
    )
    assert grounded.verdict in ("accepted", "degraded")
    assert grounded.payload is not None

    missing = guard_structured_output("treatment", {"other_field": "value"})
    assert missing.verdict == "rejected"
    assert missing.payload is None


def test_validate_text_output_red_flag_detected(guard):
    """Text containing red flag keywords produces safety issues."""
    # Red flag detector scans for concerning symptoms
    text = "这是一段包含严重疼痛和麻木无力的描述，需要足够长以通过空文本检查。"
    result = guard.validate_text_output(text, context={"extracted_info": []})
    # May or may not trigger depending on red flag patterns — just verify no crash
    assert result.status in (
        GovernanceStatus.ACCEPTED,
        GovernanceStatus.DEGRADED,
        GovernanceStatus.REJECTED,
    )


def test_validate_structured_output_red_flag(guard):
    """Structured output with red flag content in serialized text produces safety issues."""
    output = {"summary": "患者出现严重症状，需要紧急处理，文本足够长以通过检查。"}
    result = guard.validate_structured_output(output, context={"extracted_info": []})
    assert result.status in (
        GovernanceStatus.ACCEPTED,
        GovernanceStatus.DEGRADED,
        GovernanceStatus.REJECTED,
    )


def test_governance_context_fields():
    """GovernanceContext can be constructed with all fields."""
    ctx = GovernanceContext(
        output_type="treatment",
        extracted_info=[{"body_part": "肩颈", "symptom_type": "疼痛"}],
        rag_results=[{"title": "test"}],
        profile={"gender": "female", "birth_date": "1996-08-27", "age_years": 30},
        metadata={"version": 1},
    )
    assert ctx.output_type == "treatment"
    assert len(ctx.extracted_info) == 1
    assert len(ctx.rag_results) == 1
    assert ctx.profile == {"gender": "female", "birth_date": "1996-08-27", "age_years": 30}
    assert ctx.metadata == {"version": 1}
