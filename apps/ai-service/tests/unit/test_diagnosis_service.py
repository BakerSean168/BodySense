"""DiagnosisService tests for the single BodyState -> typed Agent boundary."""

import pytest
from pydantic_ai.models.test import TestModel

from src.agents.diagnosis_agent import create_diagnosis_agent
from src.configuration.diagnosis_agent_config import get_default_diagnosis_configuration
from src.services.diagnosis_service import DiagnosisService

CONFIG_ID = get_default_diagnosis_configuration().configuration_id


def _body_state(revision: int = 12) -> dict:
    return {
        "current_revision": revision,
        "facts": [
            {
                "id": "fact-neck-1",
                "kind": "discomfort",
                "body_region": "颈肩",
                "value": "久坐后酸胀",
                "details": {"trigger": "久坐"},
                "lifecycle_state": "active",
                "trend": "stable",
            }
        ],
        "observations": [
            {
                "id": "obs-neck-1",
                "kind": "posture_findings",
                "body_region": "头颈",
                "method": "posture_analysis",
                "value": {"label": "耳部相对肩部偏前"},
                "lifecycle_state": "active",
            }
        ],
    }


def _candidate(index: int = 1) -> dict:
    return {
        "concern_key": "region:头颈",
        "name": f"候选 {index}",
        "confidence": "中",
        "severity": "轻度",
        "evidence_strength": "中",
        "basis": "当前事实与观察存在匹配",
        "typical_symptoms": "颈肩酸胀",
        "reasoning_summary": "综合当前 BodyState 后存在一定匹配",
        "basis_fact_ids": ["fact-neck-1"],
        "basis_observation_ids": ["obs-neck-1"],
        "counterevidence_ids": [],
    }


def _agent_output(candidates: list[dict], status: str = "completed") -> dict:
    return {
        "status": status,
        "scope": "full_body",
        "summary": "当前身体状态可能涉及多个相关模式。",
        "candidates": candidates,
        "cross_concern_patterns": [],
        "information_gaps": [],
        "safety_summary": {},
    }


def _service(output: dict) -> tuple[DiagnosisService, TestModel]:
    model = TestModel(custom_output_args=output)
    service = DiagnosisService(diagnosis_agent=create_diagnosis_agent(model))
    return service, model


@pytest.mark.asyncio
async def test_generate_diagnosis_uses_exact_body_state_revision() -> None:
    service, model = _service(_agent_output([_candidate()]))

    result = await service.generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=_body_state(12),
        relevant_history=[{"revision": 11, "change_type": "fact.temporal_changed"}],
        profile={"age": 30},
    )

    assert result["status"] == "completed"
    assert result["candidates"][0]["basis_fact_ids"] == ["fact-neck-1"]
    assert result["agent_configuration"]["id"].startswith("diag-config-")
    assert result["agent_configuration"]["role"] == "diagnosis"
    assert "R12" in str(model.last_model_request_parameters)
    assert "fact-neck-1" in str(model.last_model_request_parameters)


@pytest.mark.asyncio
async def test_generate_diagnosis_does_not_cap_candidate_count_at_three() -> None:
    service, _ = _service(_agent_output([_candidate(i) for i in range(1, 9)]))
    result = await service.generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=_body_state(12),
    )
    assert len(result["candidates"]) == 8


@pytest.mark.asyncio
async def test_generate_diagnosis_allows_zero_candidates_when_information_is_insufficient() -> None:
    service, _ = _service(_agent_output([], status="insufficient_information"))
    result = await service.generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=_body_state(12),
    )
    assert result["status"] == "insufficient_information"
    assert result["candidates"] == []


@pytest.mark.asyncio
async def test_generate_diagnosis_blocks_current_positive_red_flag_before_agent_run() -> None:
    service, model = _service(_agent_output([_candidate()]))
    state = _body_state(12)
    state["facts"].append(
        {
            "id": "fact-safety-1",
            "kind": "discomfort",
            "body_region": "右脚",
            "value": "脚趾麻木",
            "details": {},
            "lifecycle_state": "active",
            "trend": "worsening",
        }
    )

    result = await service.generate_diagnosis(
        body_state_revision=12, configuration_id=CONFIG_ID, body_state=state
    )

    assert result["status"] == "safety_blocked"
    assert result["candidates"] == []
    assert result["agent_configuration"]["id"].startswith("diag-config-")
    assert model.last_model_request_parameters is None


@pytest.mark.asyncio
async def test_negative_or_historical_red_flag_words_do_not_block_current_diagnosis() -> None:
    service, model = _service(_agent_output([_candidate()]))
    state = _body_state(12)
    state["facts"].append(
        {
            "id": "fact-negative-1",
            "kind": "negative_finding",
            "body_region": "右脚",
            "value": "无脚趾麻木",
            "details": {},
            "lifecycle_state": "active",
            "trend": "stable",
        }
    )

    result = await service.generate_diagnosis(
        body_state_revision=12,
        configuration_id=CONFIG_ID,
        body_state=state,
        relevant_history=[{"revision": 8, "changes": {"historical": "2019 年曾扭伤"}}],
    )

    assert result["status"] == "completed"
    assert model.last_model_request_parameters is not None


@pytest.mark.asyncio
async def test_generate_diagnosis_rejects_missing_body_state() -> None:
    service, _ = _service(_agent_output([_candidate()]))
    with pytest.raises(ValueError, match="body_state is required"):
        await service.generate_diagnosis(
            body_state_revision=12, configuration_id=CONFIG_ID, body_state={}
        )


@pytest.mark.asyncio
async def test_generate_diagnosis_rejects_revision_mismatch() -> None:
    service, _ = _service(_agent_output([_candidate()]))
    with pytest.raises(ValueError, match="does not match"):
        await service.generate_diagnosis(
            body_state_revision=13,
            configuration_id=CONFIG_ID,
            body_state=_body_state(12),
        )
