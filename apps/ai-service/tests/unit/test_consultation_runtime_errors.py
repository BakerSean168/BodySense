from __future__ import annotations

from unittest.mock import patch

import pytest

from src.ai.errors import GatewayUnavailableError
from src.runtime.consultation_thread import llm_turn


@pytest.mark.asyncio
async def test_llm_turn_degrades_only_when_gateway_is_unavailable() -> None:
    emitted: list[dict[str, object]] = []
    state = {
        "runtime_messages": [{"role": "user", "content": "我的肩膀疼"}],
        "tool_rounds": 0,
    }

    with patch(
        "src.runtime.consultation_thread._get_ai_service",
        side_effect=GatewayUnavailableError("gateway unavailable"),
    ):
        result = await llm_turn(state, writer=emitted.append)

    assert result["llm_available"] is False
    assert result["accumulated_text"]
    assert any(event.get("type") == "text_delta" for event in emitted)


@pytest.mark.asyncio
async def test_llm_turn_does_not_hide_unexpected_runtime_errors() -> None:
    with patch(
        "src.runtime.consultation_thread._get_ai_service",
        side_effect=RuntimeError("programming defect"),
    ):
        with pytest.raises(RuntimeError, match="programming defect"):
            await llm_turn(
                {
                    "runtime_messages": [{"role": "user", "content": "hello"}],
                    "tool_rounds": 0,
                },
                writer=lambda _event: None,
            )
