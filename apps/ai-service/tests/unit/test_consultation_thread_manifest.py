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
                profile={"age": 30},
                extracted_info=[],
                phase="collecting",
            ):
                captured_events.append(event)

    import asyncio
    asyncio.run(run())

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
