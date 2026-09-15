"""Focused learning tests for the PydanticAI Diagnosis execution boundary."""

import pytest
from pydantic_ai import (
    ToolCallPart,
    ToolReturnPart,
    capture_run_messages,
)
from pydantic_ai.models.test import TestModel

from src.agents.diagnosis_agent import create_diagnosis_agent
from src.agents.evidence import DiagnosisEvidenceAcquirer
from src.models.dependencies import EvidenceSearcher
from src.models.diagnosis import DiagnosisAgentOutput, DiagnosisDependencies
from src.models.evidence import EvidenceBudget, EvidenceRetrievalStatus, EvidenceSearchOutcome


def _deps(evidence_searcher: EvidenceSearcher | None = None) -> DiagnosisDependencies:
    evidence_acquirer = DiagnosisEvidenceAcquirer(
        searcher=evidence_searcher,
        budget=EvidenceBudget(max_searches=2, max_results_per_search=5),
    )
    return DiagnosisDependencies(
        body_state_revision=12,
        body_state={
            "current_revision": 12,
            "facts": [
                {
                    "id": "fact-neck-1",
                    "kind": "discomfort",
                    "body_region": "颈肩",
                    "value": "久坐后酸胀",
                }
            ],
            "observations": [],
        },
        relevant_history=[{"revision": 11, "change_type": "fact.temporal_changed"}],
        profile={"gender": "female", "birth_date": "1996-08-27", "age_years": 30},
        evidence_searcher=evidence_searcher,
        evidence_acquirer=evidence_acquirer,
    )


@pytest.mark.asyncio
async def test_agent_returns_typed_diagnosis_output_without_manual_json_parsing() -> None:
    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )
    agent = create_diagnosis_agent(model)

    result = await agent.run(
        "Synthesize possible diagnosis candidates from the supplied durable state.",
        deps=_deps(evidence_searcher=FakeEvidenceSearcher()),
    )

    assert isinstance(result.output, DiagnosisAgentOutput)
    assert result.output.candidates[0].basis_fact_ids == ["fact-neck-1"]


@pytest.mark.asyncio
async def test_agent_run_context_receives_exact_body_state_revision() -> None:
    model = TestModel(
        custom_output_args={
            "status": "insufficient_information",
            "scope": "full_body",
            "summary": "现有信息不足。",
            "candidates": [],
            "cross_concern_patterns": [],
            "information_gaps": ["需要更多颈肩活动诱发信息"],
            "safety_summary": {},
        }
    )
    agent = create_diagnosis_agent(model)

    result = await agent.run(
        "Analyze this run.", deps=_deps(evidence_searcher=FakeEvidenceSearcher())
    )

    assert result.output.status == "insufficient_information"
    request_parameters = model.last_model_request_parameters
    assert request_parameters is not None
    assert "R12" in str(request_parameters)
    assert "fact-neck-1" in str(request_parameters)


class FakeEvidenceSearcher:
    def __init__(self) -> None:
        self.calls: list[tuple[str, int]] = []

    async def search(self, query: str, *, top_k: int = 5) -> EvidenceSearchOutcome:
        self.calls.append((query, top_k))
        return EvidenceSearchOutcome(
            retrieval_status=EvidenceRetrievalStatus.RESULTS_RETURNED,
            evidence=[{"evidence_id": "evidence-1", "content": "This is a piece of evidence."}],
            published_corpus_count=1,
        )


@pytest.mark.asyncio
async def test_agent_user_fact_gap_never_calls_run_scoped_searcher() -> None:
    searcher = FakeEvidenceSearcher()

    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )

    agent = create_diagnosis_agent(model)

    result = await agent.run(
        "Analyze this run.",
        deps=_deps(evidence_searcher=searcher),
    )

    # TestModel generates a user_fact EvidenceGap. User facts are owned by the
    # user and must never be substituted by external retrieval.
    assert searcher.calls == []
    assert isinstance(result.output, DiagnosisAgentOutput)


@pytest.mark.asyncio
async def test_acquire_evidence_user_fact_result_returns_to_model() -> None:
    searcher = FakeEvidenceSearcher()

    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )

    agent = create_diagnosis_agent(model)

    with capture_run_messages() as messages:
        result = await agent.run(
            "Analyze this run.",
            deps=_deps(evidence_searcher=searcher),
        )

    tool_calls = [
        part
        for message in messages
        for part in message.parts
        if isinstance(part, ToolCallPart) and part.tool_name == "acquire_evidence"
    ]

    assert len(tool_calls) == 1
    assert tool_calls[0].tool_name == "acquire_evidence"
    assert isinstance(result.output, DiagnosisAgentOutput)

    tool_returns = [
        part
        for message in messages
        for part in message.parts
        if isinstance(part, ToolReturnPart) and part.tool_name == "acquire_evidence"
    ]

    assert len(tool_returns) == 1
    assert tool_returns[0].tool_name == "acquire_evidence"

    assert "user_input_required" in str(tool_returns[0].content)
    assert "evidence-1" not in str(tool_returns[0].content)
    assert searcher.calls == []
    assert tool_returns[0].tool_call_id == tool_calls[0].tool_call_id


@pytest.mark.asyncio
async def test_acquire_evidence_user_fact_does_not_record_external_evidence() -> None:
    searcher = FakeEvidenceSearcher()

    deps = _deps(
        evidence_searcher=searcher,
    )

    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )

    agent = create_diagnosis_agent(model)

    assert deps.retrieved_evidence == []

    await agent.run(
        "Analyze this run.",
        deps=deps,
    )

    assert deps.retrieved_evidence == []
    assert searcher.calls == []
    assert deps.evidence_acquirer is not None
    assert len(deps.evidence_acquirer.attempts) == 1
    assert deps.evidence_acquirer.attempts[0].stop_reason.value == "user_input_required"


@pytest.mark.asyncio
async def test_user_fact_acquisition_does_not_mutate_existing_retrieved_evidence() -> None:
    searcher = FakeEvidenceSearcher()

    deps = _deps(
        evidence_searcher=searcher,
    )

    deps.retrieved_evidence.append(
        {
            "evidence_id": "evidence-1",
            "content": "Existing evidence.",
        }
    )

    model = TestModel(
        custom_output_args={
            "status": "completed",
            "scope": "full_body",
            "summary": "当前状态存在一个需要关注的颈肩模式。",
            "candidates": [
                {
                    "concern_key": "region:头颈",
                    "name": "颈肩姿势负荷相关模式",
                    "confidence": "中",
                    "basis_fact_ids": ["fact-neck-1"],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
        }
    )

    agent = create_diagnosis_agent(model)

    await agent.run(
        "Analyze this run.",
        deps=deps,
    )

    assert len(deps.retrieved_evidence) == 1
    assert deps.retrieved_evidence[0]["evidence_id"] == "evidence-1"
