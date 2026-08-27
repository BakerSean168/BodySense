from unittest.mock import patch

import pytest

from src.runtime.consultation_thread import get_consultation_manifest


def test_get_consultation_manifest_defaults_to_default_config() -> None:
    manifest = get_consultation_manifest(None)
    assert manifest.role == "consultation"
    assert manifest.configuration_id == "consult-config-2bd9b46735dd693c"


def test_get_consultation_manifest_resolves_known_configuration() -> None:
    manifest = get_consultation_manifest("consult-config-2bd9b46735dd693c")
    assert manifest.configuration_id == "consult-config-2bd9b46735dd693c"
    assert manifest.prompt_revision == "consultation-prompt-v1"


def test_get_consultation_manifest_rejects_unknown_configuration() -> None:
    with pytest.raises(ValueError, match="unknown Consultation configuration_id"):
        get_consultation_manifest("consult-config-does-not-exist")


def test_stream_thread_turn_emits_agent_configuration_event() -> None:
    """The runtime must emit the immutable Agent configuration + provenance."""
    from src.runtime.consultation_thread import stream_thread_turn

    captured_events = []

    async def run():
        # Patch the graph to yield a minimal done sequence.
        class FakeGraph:
            async def astream(self, *args, **kwargs):
                if False:
                    yield
            async def aget_state(self, config):
                class S:
                    interrupts = []
                return S()

        with patch(
            "src.runtime.consultation_thread.get_runtime_graph",
            return_value=FakeGraph(),
        ):
            async for event in stream_thread_turn(
                thread_id="t1",
                conversation_id="c1",
                run_id="r1",
                user_id="u1",
                user_message="hello",
                profile={"gender": "female", "birth_date": "1996-08-27", "age_years": 30},
                extracted_info=[],
                phase="collecting",
            ):
                captured_events.append(event)

    import asyncio
    asyncio.run(run())

    assert captured_events[0].channel == "runtime"
    assert captured_events[0].type == "runtime.agent_configuration"
    config_events = [
        e for e in captured_events
        if e.channel == "runtime" and e.type == "runtime.agent_configuration"
    ]
    assert len(config_events) == 1
    payload = config_events[0].payload
    assert payload["agent_configuration"]["role"] == "consultation"
    assert payload["agent_configuration"]["id"] == "consult-config-2bd9b46735dd693c"
    assert payload["execution_provenance"]["runtime"] == "langgraph"
    assert payload["execution_provenance"]["logical_model"] == "bodysense-consultation"



def test_stream_thread_turn_emits_identity_before_interrupt() -> None:
    """Interrupted turns still establish durable execution identity first."""
    from src.runtime.consultation_thread import stream_thread_turn

    captured_events = []

    class FakeInterrupt:
        id = "interrupt-1"
        value = {"question": {"text": "哪里疼？"}, "tool_call_id": "tool-1"}

    class FakeGraph:
        async def astream(self, *args, **kwargs):
            if False:
                yield

        async def aget_state(self, config):
            class S:
                interrupts = [FakeInterrupt()]

            return S()

    async def run() -> None:
        with patch(
            "src.runtime.consultation_thread.get_runtime_graph",
            return_value=FakeGraph(),
        ):
            async for event in stream_thread_turn(
                thread_id="t1",
                conversation_id="c1",
                run_id="r1",
                user_id="u1",
                user_message="hello",
                profile={},
                extracted_info=[],
                phase="collecting",
                configuration_id="consult-config-2bd9b46735dd693c",
            ):
                captured_events.append(event)

    import asyncio

    asyncio.run(run())
    assert [event.type for event in captured_events] == [
        "runtime.agent_configuration",
        "state.interaction.required",
    ]
    assert captured_events[0].payload["agent_configuration"]["id"] == (
        "consult-config-2bd9b46735dd693c"
    )


def test_resume_thread_interrupt_emits_pinned_identity_first() -> None:
    from src.runtime.consultation_thread import resume_thread_interrupt

    manifest = get_consultation_manifest("consult-config-2bd9b46735dd693c")
    captured_events = []

    class FakeSnapshot:
        values = {"consultation_manifest": manifest}
        interrupts = []

    class FakeGraph:
        async def aget_state(self, config):
            return FakeSnapshot()

        async def astream(self, *args, **kwargs):
            if False:
                yield

    async def run() -> None:
        with patch(
            "src.runtime.consultation_thread.get_runtime_graph",
            return_value=FakeGraph(),
        ):
            async for event in resume_thread_interrupt(
                thread_id="t1",
                conversation_id="c1",
                run_id="r2",
                configuration_id=manifest.configuration_id,
                answer={"text": "右侧"},
            ):
                captured_events.append(event)

    import asyncio

    asyncio.run(run())
    assert captured_events[0].type == "runtime.agent_configuration"
    assert captured_events[0].payload["agent_configuration"]["id"] == manifest.configuration_id
    assert captured_events[-1].type == "stream.done"


def test_resume_thread_interrupt_rejects_checkpoint_configuration_mismatch() -> None:
    from src.runtime.consultation_thread import resume_thread_interrupt

    requested = get_consultation_manifest("consult-config-2bd9b46735dd693c")
    checkpoint_manifest = requested.model_copy(update={"prompt_revision": "different-prompt"})
    astream_called = False

    class FakeSnapshot:
        values = {"consultation_manifest": checkpoint_manifest}
        interrupts = []

    class FakeGraph:
        async def aget_state(self, config):
            return FakeSnapshot()

        async def astream(self, *args, **kwargs):
            nonlocal astream_called
            astream_called = True
            if False:
                yield

    async def run() -> None:
        with patch(
            "src.runtime.consultation_thread.get_runtime_graph",
            return_value=FakeGraph(),
        ):
            async for _ in resume_thread_interrupt(
                thread_id="t1",
                conversation_id="c1",
                run_id="r2",
                configuration_id=requested.configuration_id,
                answer={"text": "右侧"},
            ):
                pass

    import asyncio

    with pytest.raises(ValueError, match="resume configuration mismatch"):
        asyncio.run(run())
    assert not astream_called
