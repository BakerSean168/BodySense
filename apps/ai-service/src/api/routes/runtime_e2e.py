"""Deterministic E2E streams for the runtime routes.

The stub replaces model calls with fixture events while speaking the same
private runtime protocol. It is enabled only by ``BODYSENSE_E2E_STUB_AI=1`` in
non-production environments; every fixture import stays inside the function
that needs it so production never loads the test support modules.
"""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from typing import Any
from uuid import NAMESPACE_URL, UUID, uuid5

from ...models.consultation_runtime_event import (
    AgentConfigurationRuntimeEvent,
    ConsultationRuntimeEvent,
    ConsultationRuntimeEventPayload,
    InteractionRequiredRuntimeEvent,
    PhaseChangedRuntimeEvent,
    RedFlagDetectedRuntimeEvent,
    StreamDoneRuntimeEvent,
    StreamErrorRuntimeEvent,
    TextDeltaRuntimeEvent,
    ToolCallRuntimeEvent,
    ToolResultRuntimeEvent,
)
from ...runtime.consultation_thread import get_consultation_manifest
from ...settings import e2e_stub_ai_enabled
from ..runtime_proto_adapter import serialize_runtime_event
from .runtime_schemas import ResumeInterruptRequest, StartTurnRequest


async def _e2e_safety_screen_question(question: dict[str, Any]) -> tuple[str, dict[str, Any]]:
    """Normalize the production intake question exactly as the runtime does."""
    from ...services.agent.tools.ask_user import handle_ask_user

    normalized = await handle_ask_user(question)
    if not isinstance(normalized.content, dict) or "state_binding" not in normalized.content:
        raise RuntimeError("E2E symptom intake was not normalized")
    return f"intake-{normalized.content['state_binding']['capture_id']}", normalized.content


def _e2e_intake_tool_id(run_id: str, answer: dict[str, Any]) -> str:
    """The pending E2E interrupt's tool call: the safety screen for a form answer."""
    from ...testing_support.body_state_suggestion_fixture import SUGGESTION_ANSWER_OPTIONS

    selected = answer.get("selected")
    if (
        isinstance(selected, list)
        and len(selected) == 1
        and selected[0] in SUGGESTION_ANSWER_OPTIONS
    ):
        from ...runtime.body_state_suggestion import suggestion_tool_call_id

        return suggestion_tool_call_id(run_id)
    if not isinstance(answer.get("fields"), dict):
        return f"ask-bodyfact:{run_id}"
    from ...services.consultation_state_acquisition import safety_screen_capture_id

    return f"intake-{safety_screen_capture_id(f'bodyfact-intake-v1:{run_id}', 0)}"


def _e2e_stub_enabled() -> bool:
    return e2e_stub_ai_enabled()


def _stub_event(
    *,
    seq: int,
    run_id: str,
    conversation_id: str,
    event: ConsultationRuntimeEventPayload,
    tool_call_id: str | None = None,
) -> str:
    return serialize_runtime_event(
        ConsultationRuntimeEvent(
            seq=seq,
            conversation_id=conversation_id,
            run_id=run_id,
            event=event,
            tool_call_id=tool_call_id,
        )
    )


async def e2e_turn_stream(request: StartTurnRequest) -> AsyncIterator[str]:
    if "E2E_HOLD_RUN_LONG" in request.input.text:
        await asyncio.sleep(15)
    elif "E2E_HOLD_RUN" in request.input.text:
        await asyncio.sleep(0.75)
    trigger_safety = "E2E_TRIGGER_SAFETY" in request.input.text
    seq = 1
    manifest = get_consultation_manifest(request.configuration_id)
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=AgentConfigurationRuntimeEvent(
            agent_configuration=manifest.provenance(),
            execution_provenance={
                "status": "executed",
                "runtime": "langgraph-e2e-stub",
                "logical_model": manifest.logical_model,
                "model_group_revision": manifest.model_group_revision,
                "usage": {},
            },
        ),
    )
    seq += 1
    intake_question: dict[str, Any] | None = None
    if manifest.intake and manifest.intake.policy_revision == "body-fact-intake-v1":
        from ...testing_support.body_fact_fixture import body_fact_fixture_events

        fixture_events, turn_question = await body_fact_fixture_events(
            request.input.text,
            request.business_context.body_state,
            manifest,
            request.run_id,
        )
        if turn_question and turn_question.get("purpose") == "symptom_intake":
            intake_question = turn_question
        for item in fixture_events:
            if item["type"] == "tool_call":
                event = ToolCallRuntimeEvent(tool=item["tool"], args=item["args"])
            elif item["type"] == "tool_result":
                event = ToolResultRuntimeEvent(tool=item["tool"], result=item["result"])
            else:
                event = TextDeltaRuntimeEvent(delta=item["delta"])
            yield _stub_event(
                seq=seq,
                run_id=request.run_id,
                conversation_id=request.conversation_id,
                event=event,
                tool_call_id=item.get("id"),
            )
            seq += 1
    suggestion: dict[str, Any] | None = None
    if (
        manifest.intake
        and manifest.intake.policy_revision == "body-fact-intake-v1"
        and intake_question is None
    ):
        from ...runtime.body_state_suggestion import suggestion_tool_call_id
        from ...testing_support.body_state_suggestion_fixture import (
            body_state_suggestion_fixture,
        )

        suggestion = await body_state_suggestion_fixture(
            request.input.text,
            request.business_context.body_state,
            manifest,
            request.run_id,
            request.input.message_id,
            pending_revisions=int(any(item["type"] == "tool_result" for item in fixture_events)),
        )
    if suggestion is not None:
        # The production proposer's typed suggestion; Go binds it to the
        # cited user message or replaces it with an unverified notice.
        tool_id = suggestion_tool_call_id(request.run_id)
        placeholder = {
            "question": "要把你说过的这句话记录下来吗？",
            "answer_type": "single_choice",
            "options": ["记录", "不记录"],
            "allow_custom_input": False,
            "required": True,
        }
        yield _stub_event(
            seq=seq,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            tool_call_id=tool_id,
            event=ToolCallRuntimeEvent(tool="ask_user", args=placeholder),
        )
        yield _stub_event(
            seq=seq + 1,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            tool_call_id=tool_id,
            event=InteractionRequiredRuntimeEvent(
                interaction_id=uuid5(NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}").hex,
                question=placeholder,
                suggestion=suggestion,
            ),
        )
        return
    if intake_question is not None or request.input.text in {
        "E2E_BODYFACT_ASK_SELF",
        "E2E_BODYFACT_ASK_CHOICE",
    }:
        native_id = uuid5(NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}").hex
        tool_id = f"ask-bodyfact:{request.run_id}"
        choice = request.input.text.endswith("CHOICE")
        question: dict[str, Any] = {
            "question": "Which side?" if choice else "Describe your own current symptoms.",
            "answer_type": "single_choice" if choice else "text",
            "allow_custom_input": False if choice else True,
        }
        if choice:
            question["options"] = ["left", "right"]
        if intake_question is not None:
            # The production node asked the safety screen for the
            # symptom it just proposed; Go binds it to the commit.
            tool_id, question = await _e2e_safety_screen_question(intake_question)
        yield _stub_event(
            seq=seq,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            tool_call_id=tool_id,
            event=ToolCallRuntimeEvent(tool="ask_user", args=question),
        )
        yield _stub_event(
            seq=seq + 1,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            tool_call_id=tool_id,
            event=InteractionRequiredRuntimeEvent(interaction_id=native_id, question=question),
        )
        return
    if trigger_safety:
        yield _stub_event(
            seq=seq,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            event=RedFlagDetectedRuntimeEvent(
                has_red_flags=True,
                flags=[
                    {
                        "category": "weakness",
                        "severity": "high",
                        "message": "E2E deterministic safety signal",
                    }
                ],
            ),
        )
        seq += 1
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=TextDeltaRuntimeEvent(delta="E2E consultation completed."),
    )
    seq += 1
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=PhaseChangedRuntimeEvent(
            from_phase=request.business_context.runtime_state.phase,
            to="ready_for_analysis",
            reason="e2e deterministic completion",
        ),
    )
    seq += 1
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=StreamDoneRuntimeEvent(
            usage={"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}
        ),
    )


async def e2e_resume_stream(request: ResumeInterruptRequest) -> AsyncIterator[str]:
    expected = uuid5(NAMESPACE_URL, f"bodysense:e2e:ask:{request.run_id}")
    if UUID(request.interrupt_id) != expected:
        yield _stub_event(
            seq=1,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            event=StreamErrorRuntimeEvent(message="Synthetic native interrupt mismatch."),
        )
        return
    manifest = get_consultation_manifest(request.configuration_id)
    seq = 1
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=AgentConfigurationRuntimeEvent(
            agent_configuration=manifest.provenance(),
            execution_provenance={
                "status": "executed",
                "runtime": "langgraph-e2e-stub",
                "logical_model": manifest.logical_model,
                "model_group_revision": manifest.model_group_revision,
                "usage": {},
            },
        ),
    )
    seq += 1
    from ...testing_support.body_fact_fixture import body_fact_fixture_events

    # A safety-screen form answer is not a free-text statement; like the
    # production resume, it skips statement extraction.
    answer_events: list[dict[str, Any]] = []
    if not isinstance(request.answer.get("fields"), dict):
        answer_events, _ = await body_fact_fixture_events(
            str(request.answer.get("text", "")),
            request.business_context.body_state,
            manifest,
            request.run_id,
            interrupt_id=request.interrupt_id,
        )
    for item in answer_events:
        if item["type"] == "tool_call":
            event = ToolCallRuntimeEvent(tool=item["tool"], args=item["args"])
        elif item["type"] == "tool_result":
            event = ToolResultRuntimeEvent(tool=item["tool"], result=item["result"])
        else:
            event = TextDeltaRuntimeEvent(delta=item["delta"])
        yield _stub_event(
            seq=seq,
            run_id=request.run_id,
            conversation_id=request.conversation_id,
            event=event,
            tool_call_id=item.get("id"),
        )
        seq += 1
    yield _stub_event(
        seq=seq,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        tool_call_id=_e2e_intake_tool_id(request.run_id, request.answer),
        event=ToolResultRuntimeEvent(tool="ask_user", result={"answer": request.answer}),
    )
    yield _stub_event(
        seq=seq + 1,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=TextDeltaRuntimeEvent(delta="E2E accepted answer processed."),
    )
    yield _stub_event(
        seq=seq + 2,
        run_id=request.run_id,
        conversation_id=request.conversation_id,
        event=StreamDoneRuntimeEvent(),
    )
