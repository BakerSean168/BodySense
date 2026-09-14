from __future__ import annotations

import json
from pathlib import Path

import pytest

from src.api.runtime_proto_adapter import (
    RuntimeCommandError,
    RuntimeEventError,
    parse_resume_interrupt_command,
    parse_start_turn_command,
    runtime_event_to_proto,
    serialize_runtime_event,
)
from src.models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    AnswerAttributionAddedRuntimeEvent,
    CitationAddedRuntimeEvent,
    ConsultationRuntimeEvent,
    ExtractedInfoRuntimeEvent,
    InteractionRequiredRuntimeEvent,
    KnowledgeGapRuntimeEvent,
    LifestyleContextRuntimeEvent,
    PhaseChangedRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    SafetyOutputRejectedRuntimeEvent,
    SafetyOutputReviewedRuntimeEvent,
    StreamDoneRuntimeEvent,
    StreamErrorRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
    UsageReportedRuntimeEvent,
)

ROOT = Path(__file__).resolve().parents[4]
CORPUS = json.loads(
    (ROOT / "contracts/internal/agent-runtime/fixtures/runtime.v1.json").read_text()
)


def test_start_turn_proto_boundary_projects_to_runtime_input() -> None:
    command = CORPUS["start_turn"]
    runtime_input = parse_start_turn_command(command["thread_id"], command)
    assert "thread_id" not in runtime_input
    assert runtime_input["run_id"] == command["run_id"]
    assert runtime_input["configuration_id"] == command["configuration_id"]
    assert runtime_input["business_context"]["runtime_state"]["phase"] == "collecting"


def test_start_turn_proto_boundary_rejects_path_identity_mismatch() -> None:
    with pytest.raises(RuntimeCommandError, match="thread identity mismatch"):
        parse_start_turn_command("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", CORPUS["start_turn"])


def test_resume_proto_boundary_projects_to_runtime_input() -> None:
    command = CORPUS["resume_interrupt"]
    runtime_input = parse_resume_interrupt_command(
        command["thread_id"], command["interrupt_id"], command
    )
    assert runtime_input["interrupt_id"] == command["interrupt_id"]
    assert runtime_input["answer"] == {"value": "yes"}


def test_resume_proto_boundary_rejects_interrupt_path_mismatch() -> None:
    command = CORPUS["resume_interrupt"]
    with pytest.raises(RuntimeCommandError, match="interrupt identity mismatch"):
        parse_resume_interrupt_command(
            command["thread_id"],
            "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            command,
        )

def _typed_event_from_proto_fixture(value: dict) -> tuple[str, ConsultationRuntimeEvent]:
    variant = next(
        key for key in value if key not in {"version", "seq", "ids"}
    )
    payload = value[variant]

    if variant == "agent_configuration":
        event_payload = AgentConfigurationRuntimeEvent(
            agent_configuration=payload["agent_configuration"],
            execution_provenance=payload["execution_provenance"],
        )
    elif variant == "text_delta":
        event_payload = TextDeltaRuntimeEvent(delta=payload["delta"])
    elif variant == "tool_call":
        event_payload = ToolCallRuntimeEvent(tool=payload["tool"], args=payload["args"])
    elif variant == "tool_result":
        event_payload = ToolResultRuntimeEvent(tool=payload["tool"], result=payload["result"])
    elif variant == "extracted_info":
        event_payload = ExtractedInfoRuntimeEvent(info=payload["info"])
    elif variant == "lifestyle_context":
        event_payload = LifestyleContextRuntimeEvent(context=payload["context"])
    elif variant == "interaction_required":
        event_payload = InteractionRequiredRuntimeEvent(
            interaction_id=payload["interaction_id"],
            question=payload["question"],
        )
    elif variant == "phase_changed":
        event_payload = PhaseChangedRuntimeEvent(
            from_phase=payload.get("from"),
            to=payload["to"],
            reason=payload["reason"],
        )
    elif variant == "citation_added":
        event_payload = CitationAddedRuntimeEvent(citation=payload["citation"])
    elif variant == "answer_attribution_added":
        event_payload = AnswerAttributionAddedRuntimeEvent(
            attribution=payload["attribution"]
        )
    elif variant == "knowledge_gap":
        event_payload = KnowledgeGapRuntimeEvent(
            query=payload["query"],
            message=payload["message"],
        )
    elif variant == "red_flag_detected":
        event_payload = RedFlagDetectedRuntimeEvent(
            has_red_flags=payload["has_red_flags"],
            flags=payload["flags"],
        )
    elif variant == "output_reviewed":
        event_payload = SafetyOutputReviewedRuntimeEvent(
            kind=payload["kind"],
            verdict=payload["verdict"],
            reasons=payload["reasons"],
            issues=payload["issues"],
            safety_fallback=payload.get("safety_fallback"),
        )
    elif variant == "output_rejected":
        event_payload = SafetyOutputRejectedRuntimeEvent(
            kind=payload["kind"],
            verdict=payload["verdict"],
            reasons=payload["reasons"],
            issues=payload["issues"],
            safety_fallback=payload.get("safety_fallback"),
        )
    elif variant == "usage_reported":
        event_payload = UsageReportedRuntimeEvent(usage=payload["usage"])
    elif variant == "stream_done":
        event_payload = StreamDoneRuntimeEvent(
            response_id=payload.get("response_id"),
            usage=payload.get("usage"),
            governance=payload.get("governance"),
        )
    elif variant == "stream_error":
        event_payload = StreamErrorRuntimeEvent(message=payload["message"])
    else:
        raise AssertionError(f"fixture has unsupported runtime variant: {variant}")

    ids = value["ids"]
    return variant, ConsultationRuntimeEvent(
        seq=int(value["seq"]),
        conversation_id=ids["conversation_id"],
        run_id=ids["run_id"],
        tool_call_id=ids.get("tool_call_id"),
        event=event_payload,
    )


def test_all_python_runtime_event_variants_project_to_proto_oneof() -> None:
    assert len(CORPUS["events"]) == 17
    for fixture in CORPUS["events"]:
        expected_variant, event = _typed_event_from_proto_fixture(fixture)
        proto = runtime_event_to_proto(event)
        assert proto.WhichOneof("event") == expected_variant

        wire = json.loads(serialize_runtime_event(event))
        assert expected_variant in wire
        assert wire[expected_variant] == fixture[expected_variant]
        assert wire["ids"] == fixture["ids"]
        assert "channel" not in wire
        assert "type" not in wire
        assert "payload" not in wire


def test_python_runtime_event_boundary_rejects_missing_run_identity() -> None:
    event = ConsultationRuntimeEvent(
        seq=1,
        conversation_id="33333333-3333-4333-8333-333333333333",
        run_id="",
        event=TextDeltaRuntimeEvent(delta="hello"),
    )
    with pytest.raises(RuntimeEventError, match="invalid private runtime event"):
        runtime_event_to_proto(event)


def test_python_runtime_event_boundary_rejects_invalid_typed_payload() -> None:
    event = ConsultationRuntimeEvent(
        seq=1,
        conversation_id="33333333-3333-4333-8333-333333333333",
        run_id="22222222-2222-4222-8222-222222222222",
        event=KnowledgeGapRuntimeEvent(query="", message=""),
    )
    with pytest.raises(RuntimeEventError, match="invalid private runtime event"):
        runtime_event_to_proto(event)
