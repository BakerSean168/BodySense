"""Tests for AIOutputGuard."""

import pytest

from src.services.governance.output_guard import AIOutputGuard
from src.services.governance.types import GovernanceStatus


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
