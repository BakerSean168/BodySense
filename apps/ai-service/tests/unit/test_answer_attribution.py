from types import SimpleNamespace

import pytest

from src.services.agent.answer_attribution import (
    ANSWER_ATTRIBUTION_POLICY_REVISION,
    build_published_evidence_binding,
    validate_and_evaluate_attribution,
)


def _published_result():
    return SimpleNamespace(
        id=517,
        title="疼痛与伤害感受 · 一句话定义",
        summary="疼痛与伤害感受不是同一现象",
        body_markdown=(
            "疼痛是不愉快的感觉与情绪体验；nociception 是神经系统对有害刺激的编码过程，"
            "二者不是同一现象。"
        ),
        source_type="thought_forest_note",
        source_key="thought-forest:z/pain-and-nociception.md",
        unit_key="tfu-18d2dc2e4effd2c62097d44794f76e29",
        lifecycle_status="published",
        review_status="reviewed",
        publication_id="11111111-1111-1111-1111-111111111111",
        publication_key="pain-definition-v3",
        publication_batch_key="thought-forest-reviewed-health-pilot",
        published_version=3,
        unit_metadata={
            "source_locator": {
                "locator_type": "markdown_lines",
                "repository": "thought-forest",
                "git_commit": "abc123",
                "path": "z/pain-and-nociception.md",
                "line_start": 20,
                "line_end": 23,
            },
            "claim_candidate": {
                "claim_id": "tfc-a0ffebe00c719febbe6b7a3cd3f7e70b",
                "claim_kind": "definition",
            },
            "claim_review": {
                "review_id": "pain-definition-claim-review-pilot-2026-08-23",
            },
        },
    )


def test_build_published_evidence_binding_uses_publication_identity():
    binding = build_published_evidence_binding(_published_result())
    assert binding is not None
    assert binding["evidence_ref"] == (
        "published:11111111-1111-1111-1111-111111111111:v3:"
        "tfu-18d2dc2e4effd2c62097d44794f76e29"
    )
    assert binding["publication_key"] == "pain-definition-v3"
    assert binding["claim_id"] == "tfc-a0ffebe00c719febbe6b7a3cd3f7e70b"
    assert binding["claim_review_id"] == "pain-definition-claim-review-pilot-2026-08-23"
    assert binding["source_locator"]["line_start"] == 20


def test_non_published_or_non_thought_forest_result_has_no_runtime_binding():
    result = _published_result()
    result.lifecycle_status = "reviewed"
    assert build_published_evidence_binding(result) is None
    result.lifecycle_status = "published"
    result.source_type = "video"
    assert build_published_evidence_binding(result) is None


def test_validate_and_evaluate_attribution_accepts_only_retrieved_refs():
    binding = build_published_evidence_binding(_published_result())
    assert binding is not None
    ref = binding["evidence_ref"]
    output = validate_and_evaluate_attribution(
        [{"claim_text": "疼痛与伤害感受不是同一现象", "evidence_refs": [ref]}],
        {ref: binding},
    )
    assert len(output) == 1
    assert output[0]["policy_revision"] == ANSWER_ATTRIBUTION_POLICY_REVISION
    assert output[0]["grounding_status"] == "supported"
    assert output[0]["bindings"][0]["publication_key"] == "pain-definition-v3"

    with pytest.raises(ValueError, match="not retrieved in this turn"):
        validate_and_evaluate_attribution(
            [{"claim_text": "疼痛是一种体验", "evidence_refs": ["published:unknown"]}],
            {ref: binding},
        )


def test_attribution_grounding_degrades_or_rejects_when_claim_is_not_supported():
    binding = build_published_evidence_binding(_published_result())
    assert binding is not None
    ref = binding["evidence_ref"]

    degraded = validate_and_evaluate_attribution(
        [{"claim_text": "疼痛需要结合体验理解", "evidence_refs": [ref]}],
        {ref: binding},
    )[0]
    assert degraded["grounding_status"] in {"supported", "degraded"}

    rejected = validate_and_evaluate_attribution(
        [{"claim_text": "PostgreSQL 索引一定要使用 B-tree", "evidence_refs": [ref]}],
        {ref: binding},
    )[0]
    assert rejected["grounding_status"] == "rejected"


def test_attribution_contract_is_bounded():
    binding = build_published_evidence_binding(_published_result())
    assert binding is not None
    ref = binding["evidence_ref"]
    claims = [
        {"claim_text": f"疼痛定义 {index}", "evidence_refs": [ref]}
        for index in range(7)
    ]
    with pytest.raises(ValueError, match="at most 6 claims"):
        validate_and_evaluate_attribution(claims, {ref: binding})


@pytest.mark.asyncio
async def test_consultation_runtime_emits_attribution_event_only_for_current_turn_ref():
    from src.runtime.consultation_thread import execute_tool

    binding = build_published_evidence_binding(_published_result())
    assert binding is not None
    ref = binding["evidence_ref"]
    events: list[dict] = []
    state = {
        "runtime_messages": [],
        "pending_tool_calls": [
            {
                "id": "tool-attribution-1",
                "name": "record_answer_attribution",
                "arguments": {
                    "claims": [
                        {
                            "claim_text": "疼痛与伤害感受不是同一现象",
                            "evidence_refs": [ref],
                        }
                    ]
                },
            }
        ],
        "retrieved_published_evidence": {ref: binding},
        "answer_attributions": [],
    }

    update = await execute_tool(state, writer=events.append)

    attribution_events = [event for event in events if event["type"] == "answer_attribution"]
    assert len(attribution_events) == 1
    attribution = attribution_events[0]["attribution"]
    assert attribution["attribution_id"] == "tool-attribution-1:0"
    assert attribution["grounding_status"] == "supported"
    assert update["answer_attributions"][0]["evidence_refs"] == [ref]
    assert any(
        event["type"] == "tool_result" and event["result"]["status"] == "ok"
        for event in events
    )


@pytest.mark.asyncio
async def test_consultation_runtime_rejects_stale_or_invented_attribution_ref():
    from src.runtime.consultation_thread import execute_tool

    events: list[dict] = []
    state = {
        "runtime_messages": [],
        "pending_tool_calls": [
            {
                "id": "tool-attribution-2",
                "name": "record_answer_attribution",
                "arguments": {
                    "claims": [
                        {
                            "claim_text": "疼痛是一种体验",
                            "evidence_refs": ["published:invented"],
                        }
                    ]
                },
            }
        ],
        "retrieved_published_evidence": {},
        "answer_attributions": [],
    }

    update = await execute_tool(state, writer=events.append)

    assert not any(event["type"] == "answer_attribution" for event in events)
    error_results = [
        event
        for event in events
        if event["type"] == "tool_result" and event["result"]["status"] == "error"
    ]
    assert len(error_results) == 1
    assert "not retrieved in this turn" in error_results[0]["result"]["error"]
    assert "answer_attributions" not in update
