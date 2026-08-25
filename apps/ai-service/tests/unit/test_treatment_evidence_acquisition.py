"""EvidenceGap policy tests for the immutable Treatment v2 Challenger."""

from __future__ import annotations

from typing import Any

import pytest
from pydantic_ai import ModelResponse, ToolCallPart, ToolReturnPart
from pydantic_ai.models.function import AgentInfo, FunctionModel
from pydantic_ai.models.test import TestModel

from src.agents.evidence import TreatmentEvidenceAcquirer
from src.agents.treatment_agent import (
    TREATMENT_TOOL_POLICY_REVISION,
    TREATMENT_TOOL_POLICY_V2,
    create_treatment_agent,
    treatment_tool_names,
)
from src.configuration.treatment_agent_config import CONFIG_ROOT, load_manifest
from src.models.evidence import (
    EvidenceAcquisitionStatus,
    EvidenceBudget,
    EvidenceGap,
    EvidenceGapKind,
    EvidenceRetrievalStatus,
    EvidenceSearchOutcome,
    EvidenceStopReason,
)
from src.models.treatment import TreatmentDependencies
from src.services.treatment_agent_service import TreatmentAgentService


class FakeEvidenceSearcher:
    def __init__(self) -> None:
        self.calls: list[tuple[str, int]] = []

    async def search(self, query: str, *, top_k: int = 5) -> EvidenceSearchOutcome:
        self.calls.append((query, top_k))
        return EvidenceSearchOutcome(
            retrieval_status=EvidenceRetrievalStatus.RESULTS_RETURNED,
            evidence=[{"evidence_id": f"evidence-{len(self.calls)}", "summary": query}],
            published_corpus_count=1,
        )


def _gap(
    gap_id: str,
    *,
    kind: EvidenceGapKind = EvidenceGapKind.EXTERNAL_KNOWLEDGE,
    critical: bool = False,
) -> EvidenceGap:
    return EvidenceGap(
        gap_id=gap_id,
        kind=kind,
        description=f"缺少会改变 intervention 的信息 {gap_id}",
        rationale="该信息会改变干预选择或剂量",
        critical=critical,
        query=None if kind == EvidenceGapKind.USER_FACT else f"targeted treatment query {gap_id}",
    )


def _v2_config():
    return load_manifest(CONFIG_ROOT / "treatment-v2-evidence-gap.yaml")


def _proposal_output() -> dict[str, Any]:
    return {
        "status": "proposed",
        "summary": "低风险渐进方案。",
        "goal": "改善负荷耐受",
        "duration_weeks": 4,
        "interventions": [
            {
                "kind": "exercise",
                "title": "渐进活动",
                "description": "在可耐受范围内进行。",
                "prescription": {"sets": 2, "reps": 8},
            }
        ],
        "daily_habits": [],
        "expected_timeline": "2 至 4 周复核",
        "warning_signs": [],
        "review_triggers": ["症状明显加重"],
        "safety_notes": [],
        "evidence_ids": [],
    }


@pytest.mark.asyncio
async def test_treatment_user_fact_gap_never_calls_external_search() -> None:
    searcher = FakeEvidenceSearcher()
    acquirer = TreatmentEvidenceAcquirer(searcher=searcher, budget=EvidenceBudget(max_searches=2))

    result = await acquirer.acquire(_gap("trigger", kind=EvidenceGapKind.USER_FACT))

    assert searcher.calls == []
    assert result.attempt.search_performed is False
    assert result.attempt.stop_reason == EvidenceStopReason.USER_INPUT_REQUIRED
    assert result.budget["used_searches"] == 0
    assert acquirer.trace().external_evidence_status.value == "not_required"


@pytest.mark.asyncio
async def test_treatment_external_gap_is_bounded_and_audited() -> None:
    searcher = FakeEvidenceSearcher()
    acquirer = TreatmentEvidenceAcquirer(searcher=searcher, budget=EvidenceBudget(max_searches=1))

    first = await acquirer.acquire(_gap("dose"), top_k=9)
    second_gap = _gap("progression", critical=True)
    second = await acquirer.acquire(second_gap)

    assert searcher.calls == [("targeted treatment query dose", 5)]
    assert first.attempt.status == EvidenceAcquisitionStatus.EVIDENCE_RETURNED
    assert first.attempt.evidence_ids == ["evidence-1"]
    assert second.attempt.stop_reason == EvidenceStopReason.BUDGET_EXHAUSTED
    assert acquirer.trace().unresolved_critical_gaps == [second_gap]
    assert acquirer.trace().external_evidence_status.value == "partially_available"


@pytest.mark.asyncio
async def test_treatment_v2_exposes_only_gap_aware_tool() -> None:
    config = _v2_config()
    model = TestModel(call_tools=[], custom_output_args=_proposal_output())
    agent = create_treatment_agent(
        model,
        prompt_revision=config.prompt_revision,
        output_schema_revision=config.output_schema_revision,
        tool_policy_revision=config.tool_policy_revision,
        evidence_policy_revision=config.evidence_policy_revision,
    )
    deps = TreatmentDependencies(
        user_id="user-1",
        body_state_revision=1,
        body_state={"current_revision": 1},
        diagnosis_analysis={"analysis_id": "analysis-1"},
        candidate_assessments=[{"candidate_id": "candidate-1", "state": "confirmed"}],
        evidence_acquirer=TreatmentEvidenceAcquirer(searcher=None, budget=EvidenceBudget()),
    )

    await agent.run("Propose.", deps=deps)

    parameters = model.last_model_request_parameters
    assert parameters is not None
    assert treatment_tool_names(TREATMENT_TOOL_POLICY_REVISION) == ["search_evidence"]
    assert treatment_tool_names(TREATMENT_TOOL_POLICY_V2) == ["acquire_evidence"]
    assert [tool.name for tool in parameters.function_tools] == ["acquire_evidence"]
    schema = parameters.function_tools[0].parameters_json_schema
    assert "gap" in schema["properties"]
    assert "query" not in schema["properties"]


@pytest.mark.asyncio
async def test_treatment_service_records_budget_exhaustion_trace() -> None:
    config = _v2_config()
    searcher = FakeEvidenceSearcher()

    def model_function(messages: list[Any], info: AgentInfo) -> ModelResponse:
        returns = [
            part
            for message in messages
            for part in message.parts
            if isinstance(part, ToolReturnPart) and part.tool_name == "acquire_evidence"
        ]
        if len(returns) < 3:
            index = len(returns) + 1
            gap = _gap(f"{index}", critical=index == 3)
            return ModelResponse(
                parts=[
                    ToolCallPart(
                        "acquire_evidence",
                        {"gap": gap.model_dump(mode="json"), "top_k": 5},
                        tool_call_id=f"gap-{index}",
                    )
                ]
            )
        output_tool = info.output_tools[0]
        return ModelResponse(
            parts=[ToolCallPart(output_tool.name, _proposal_output(), tool_call_id="final-output")]
        )

    service = TreatmentAgentService(
        model_resolver=lambda _config: FunctionModel(model_function),
        evidence_searcher_factory=lambda _user_id: searcher,
    )
    result = await service.recommend(
        user_id="treatment-evidence-user",
        body_state_revision=12,
        configuration_id=config.configuration_id,
        body_state={"current_revision": 12},
        diagnosis_analysis={"analysis_id": "analysis-1", "status": "completed"},
        candidate_assessments=[{"candidate_id": "candidate-1", "state": "confirmed"}],
        profile={},
        user_constraints={},
        evidence=[],
    )

    assert searcher.calls == [
        ("targeted treatment query 1", 5),
        ("targeted treatment query 2", 5),
    ]
    trace = result["evidence_acquisition"]
    assert trace["policy_revision"] == "treatment-evidence-gap-v2"
    assert trace["budget"]["used_searches"] == 2
    assert len(trace["attempts"]) == 3
    assert trace["attempts"][2]["stop_reason"] == "budget_exhausted"
    assert trace["unresolved_critical_gaps"][0]["gap_id"] == "3"
    assert trace["external_evidence_status"] == "partially_available"
