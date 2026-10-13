import asyncio
from pathlib import Path
from unittest.mock import AsyncMock

import pytest
from pydantic import ValidationError
from pydantic_ai.models.test import TestModel

from src.configuration.consultation_agent_config import load_manifest
from src.models.body_fact_intake import (
    BodyFactDraft,
    BodyFactIntakeOutput,
    body_fact_proposal,
    validate_fact_proposal,
)
from src.runtime.body_fact_capture import acquire_body_fact_turn
from src.services.body_fact_intake_service import BodyFactIntakeService


def configuration():
    return load_manifest(Path("config/agents/consultation-v3.yaml"))


def output(text="我没有肩痛", presence="absent"):
    return BodyFactIntakeOutput(
        turn_kind="self_report",
        facts=[BodyFactDraft(kind="symptom", statement=text, subject="self", presence=presence)],
    )


@pytest.mark.parametrize(
    "text", ["我没有肩痛", "我昨天肩膀不舒服", "I have no shoulder pain", "My shoulder hurts"]
)
def test_full_self_assertions_preserve_original_and_unknown_time(text):
    proposal = body_fact_proposal(output(text, "unknown"), text, 7)
    assert proposal["status"] == "proposed"
    assert proposal["expected_revision"] == 7
    assert proposal["facts"][0]["statement"] == text
    assert proposal["facts"][0]["temporal"] == {"precision": "unknown"}
    assert "receipt" not in proposal
    assert "message_id" not in proposal


@pytest.mark.parametrize(
    "text",
    [
        "我的朋友肩膀痛",
        "我妈妈没有肩痛",
        "如果我肩痛",
        "我想知道肩痛怎么办",
        "My friend has shoulder pain",
    ],
)
def test_third_party_hypothetical_and_general_question_cannot_be_self_facts(text):
    with pytest.raises(ValueError):
        validate_fact_proposal(output(text, "unknown"), text)


def test_quote_and_truncated_negation_cannot_be_lifted_out_of_context():
    for original, statement in [
        ("朋友说“我没有肩痛”", "我没有肩痛"),
        ("我肩痛是假设，并不是真的", "我肩痛"),
        ("I have no shoulder pain", "shoulder pain"),
    ]:
        with pytest.raises(ValueError):
            validate_fact_proposal(output(statement, "unknown"), original)
    with pytest.raises(ValueError):
        validate_fact_proposal(output("我没有肩痛", "present"), "我没有肩痛")


def test_correction_general_and_unknown_revision_fail_closed():
    with pytest.raises(ValidationError):
        BodyFactIntakeOutput(turn_kind="correction", facts=output().facts)
    with pytest.raises(ValidationError):
        BodyFactIntakeOutput(turn_kind="general_question", facts=output().facts)
    with pytest.raises(ValueError):
        body_fact_proposal(output(), "我没有肩痛", True)


def test_actual_intake_agent_validates_generated_output_before_proposal():
    async def run():
        generated = output().model_dump(mode="json")
        service = BodyFactIntakeService(
            model_resolver=lambda _: TestModel(custom_output_args=generated)
        )
        parsed = await service.assess(latest_user_message="我没有肩痛", config=configuration())
        assert parsed.facts[0].presence == "absent"
        assert parsed.facts[0].statement == "我没有肩痛"

    asyncio.run(run())


def test_runtime_emits_one_proposal_not_fake_persistence_or_legacy_writes():
    async def run():
        writer = []
        service = AsyncMock(spec=BodyFactIntakeService)
        service.assess.return_value = output()
        result = await acquire_body_fact_turn(
            {
                "latest_user_message": "我没有肩痛",
                "body_state": {"records_available": True, "current_revision": 4},
            },
            writer=writer.append,
            manifest=configuration(),
            intake_service=service,
        )
        assert [event["type"] for event in writer] == ["tool_call", "tool_result"]
        assert writer[0]["args"] == {"fact_count": 1}
        assert writer[1]["result"]["status"] == "proposed"
        assert writer[1]["result"]["facts"][0]["details"] == {"presence": "absent"}
        assert result["intake_question"] is None
        assert result["intake_result"]["capture_status"] == "proposed"

    asyncio.run(run())


def test_unavailable_state_does_not_assume_zero_or_call_model():
    async def run():
        writer = []
        service = AsyncMock(spec=BodyFactIntakeService)
        result = await acquire_body_fact_turn(
            {"latest_user_message": "我没有肩痛", "body_state": {}},
            writer=writer.append,
            manifest=configuration(),
            intake_service=service,
        )
        service.assess.assert_not_called()
        assert result["intake_result"]["capture_status"] == "unavailable"
        assert all(event["type"] != "tool_result" for event in writer)

    asyncio.run(run())


def test_ambiguous_message_asks_without_saving_and_failure_does_not_guess():
    async def run():
        service = AsyncMock(spec=BodyFactIntakeService)
        service.assess.return_value = BodyFactIntakeOutput(
            turn_kind="ambiguous", clarification="这指的是谁？"
        )
        state = {
            "latest_user_message": "肩膀痛",
            "body_state": {"records_available": True, "current_revision": 0},
        }
        events = []
        result = await acquire_body_fact_turn(
            state, writer=events.append, manifest=configuration(), intake_service=service
        )
        assert result["intake_question"]["question"] == "这指的是谁？"
        assert events == []
        service.assess.side_effect = ValueError("unverifiable output")
        result = await acquire_body_fact_turn(
            state, writer=events.append, manifest=configuration(), intake_service=service
        )
        assert result["intake_result"]["capture_status"] == "unavailable"
        assert all(event["type"] == "text_delta" for event in events)

    asyncio.run(run())


def test_protobuf_struct_revision_normalizes_only_lossless_integers():
    assert body_fact_proposal(output(), "我没有肩痛", 4.0)["expected_revision"] == 4
    for bad in (True, 0.5, float("nan"), float("inf"), 2**53, "4"):
        with pytest.raises(ValueError):
            body_fact_proposal(output(), "我没有肩痛", bad)


def test_question_shaped_self_reference_does_not_become_a_fact():
    for text in ("我应该跑步吗？", "我有肩痛吗", "My shoulder hurts?"):
        with pytest.raises(ValueError):
            validate_fact_proposal(output(text, "unknown"), text)
