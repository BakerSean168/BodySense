"""EvidenceGap policy tests for the Diagnosis v2 Challenger runtime."""

from __future__ import annotations

from typing import Any

import pytest
from pydantic import ValidationError
from pydantic_ai import ModelResponse, ToolCallPart, ToolReturnPart
from pydantic_ai.models.function import AgentInfo, FunctionModel
from pydantic_ai.models.test import TestModel

from src.agents.diagnosis_agent import (
    DIAGNOSIS_TOOL_POLICY_V2,
    create_diagnosis_agent,
    diagnosis_tool_names,
)
from src.agents.evidence import DiagnosisEvidenceAcquirer
from src.configuration.diagnosis_agent_config import CONFIG_ROOT, load_manifest
from src.models.diagnosis import DiagnosisDependencies
from src.models.evidence import (
    EvidenceAcquisitionStatus,
    EvidenceBudget,
    EvidenceGap,
    EvidenceGapKind,
    EvidenceRetrievalStatus,
    EvidenceSearchOutcome,
    EvidenceStopReason,
)
from src.services.diagnosis_service import DiagnosisService


class FakeEvidenceSearcher:
    def __init__(self) -> None:
        self.calls: list[tuple[str, int]] = []

    async def search(self, query: str, *, top_k: int = 5) -> EvidenceSearchOutcome:
        self.calls.append((query, top_k))
        return EvidenceSearchOutcome(
            retrieval_status=EvidenceRetrievalStatus.RESULTS_RETURNED,
            evidence=[
                {
                    "evidence_id": f"evidence-{len(self.calls)}",
                    "content": f"evidence for {query}",
                }
            ],
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
        description=f"缺少关键依据 {gap_id}",
        rationale="该信息会改变候选的支持强度",
        critical=critical,
        query=None if kind == EvidenceGapKind.USER_FACT else f"targeted query {gap_id}",
    )


def _v2_config():
    return load_manifest(CONFIG_ROOT / "diagnosis-v2-evidence-gap.yaml")


def _completed_output() -> dict[str, Any]:
    return {
        "status": "completed",
        "scope": "full_body",
        "summary": "有一个候选。",
        "candidates": [{"name": "候选", "confidence": "中"}],
        "cross_concern_patterns": [],
        "information_gaps": [],
        "safety_summary": {},
    }


def test_evidence_gap_source_contract_prevents_user_fact_search_queries() -> None:
    with pytest.raises(ValidationError, match="must not contain an external search query"):
        EvidenceGap(
            gap_id="user-trigger",
            kind=EvidenceGapKind.USER_FACT,
            description="需要用户说明什么动作诱发症状",
            rationale="这是用户自身经历，外部资料不能替代",
            query="what movement causes this user's pain",
        )
    with pytest.raises(ValidationError, match="requires a targeted query"):
        EvidenceGap(
            gap_id="external-mechanism",
            kind=EvidenceGapKind.EXTERNAL_KNOWLEDGE,
            description="需要核对外部机制证据",
            rationale="会改变候选支持强度",
        )


@pytest.mark.asyncio
async def test_user_fact_gap_never_calls_external_search() -> None:
    searcher = FakeEvidenceSearcher()
    acquirer = DiagnosisEvidenceAcquirer(searcher=searcher, budget=EvidenceBudget(max_searches=2))

    result = await acquirer.acquire(_gap("user-trigger", kind=EvidenceGapKind.USER_FACT))

    assert searcher.calls == []
    assert result.attempt.search_performed is False
    assert result.attempt.stop_reason == EvidenceStopReason.USER_INPUT_REQUIRED
    assert result.budget["used_searches"] == 0
    assert acquirer.trace().external_evidence_status.value == "not_required"


@pytest.mark.asyncio
async def test_external_gap_records_rationale_query_budget_and_evidence_identity() -> None:
    searcher = FakeEvidenceSearcher()
    acquirer = DiagnosisEvidenceAcquirer(searcher=searcher, budget=EvidenceBudget(max_searches=2))

    result = await acquirer.acquire(_gap("mechanism"), top_k=9)

    assert searcher.calls == [("targeted query mechanism", 5)]
    assert result.attempt.status == EvidenceAcquisitionStatus.EVIDENCE_RETURNED
    assert result.attempt.stop_reason == EvidenceStopReason.EVIDENCE_RETURNED
    assert result.attempt.gap.rationale == "该信息会改变候选的支持强度"
    assert result.attempt.evidence_ids == ["evidence-1"]
    assert result.budget["remaining_searches"] == 1
    assert acquirer.trace().external_evidence_status.value == "available"


@pytest.mark.asyncio
async def test_critical_gap_survives_budget_exhaustion_without_search() -> None:
    searcher = FakeEvidenceSearcher()
    acquirer = DiagnosisEvidenceAcquirer(searcher=searcher, budget=EvidenceBudget(max_searches=0))
    gap = _gap("critical", critical=True)

    result = await acquirer.acquire(gap)
    trace = acquirer.trace()

    assert searcher.calls == []
    assert result.attempt.stop_reason == EvidenceStopReason.BUDGET_EXHAUSTED
    assert trace.unresolved_critical_gaps == [gap]
    assert trace.external_evidence_status.value == "unresolved"


@pytest.mark.asyncio
async def test_v2_agent_exposes_only_gap_aware_acquisition_tool() -> None:
    config = _v2_config()
    model = TestModel(call_tools=[], custom_output_args=_completed_output())
    agent = create_diagnosis_agent(
        model,
        prompt_revision=config.prompt_revision,
        output_schema_revision=config.output_schema_revision,
        tool_policy_revision=config.tool_policy_revision,
        evidence_policy_revision=config.evidence_policy_revision,
    )
    deps = DiagnosisDependencies(
        body_state_revision=1,
        body_state={"current_revision": 1, "facts": []},
        evidence_acquirer=DiagnosisEvidenceAcquirer(
            searcher=None,
            budget=EvidenceBudget(),
        ),
    )

    await agent.run("Analyze.", deps=deps)

    parameters = model.last_model_request_parameters
    assert parameters is not None
    assert diagnosis_tool_names(DIAGNOSIS_TOOL_POLICY_V2) == ["acquire_evidence"]
    assert [tool.name for tool in parameters.function_tools] == ["acquire_evidence"]
    tool_schema = parameters.function_tools[0].parameters_json_schema
    assert "gap" in tool_schema["properties"]
    assert "query" not in tool_schema["properties"]


@pytest.mark.asyncio
async def test_service_preserves_critical_third_gap_after_two_search_budget_is_exhausted() -> None:
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
            parts=[
                ToolCallPart(
                    output_tool.name,
                    _completed_output(),
                    tool_call_id="final-output",
                )
            ]
        )

    model = FunctionModel(model_function)
    service = DiagnosisService(
        model_resolver=lambda _config: model,
        evidence_searcher_factory=lambda _user_id: searcher,
    )

    result = await service.generate_diagnosis(
        user_id="evidence-user",
        body_state_revision=12,
        configuration_id=config.configuration_id,
        body_state={
            "current_revision": 12,
            "facts": [
                {
                    "id": "fact-neck",
                    "kind": "discomfort",
                    "body_region": "颈肩",
                    "value": "轻度酸胀",
                    "details": {"severity": "轻度"},
                }
            ],
        },
    )

    assert searcher.calls == [("targeted query 1", 5), ("targeted query 2", 5)]
    trace = result["evidence_acquisition"]
    assert trace["budget"]["used_searches"] == 2
    assert len(trace["attempts"]) == 3
    assert trace["attempts"][2]["stop_reason"] == "budget_exhausted"
    assert trace["attempts"][2]["search_performed"] is False
    assert trace["external_evidence_status"] == "partially_available"
    assert result["information_gaps"] == ["缺少关键依据 3"]
    assert result["agent_configuration"]["id"] == config.configuration_id
