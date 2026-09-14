from __future__ import annotations

from unittest.mock import patch

import pytest
from pydantic_ai.exceptions import AgentRunError

from src.ai.errors import GatewayUnavailableError
from src.configuration.consultation_agent_config import get_default_consultation_configuration
from src.services.consultation_intake_service import ConsultationIntakeService


class _FailingAgent:
    def __init__(self, error: Exception) -> None:
        self._error = error

    async def run(self, *_args, **_kwargs):
        raise self._error


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "error",
    [AgentRunError("model call failed"), GatewayUnavailableError("gateway unavailable")],
)
async def test_intake_degrades_only_for_expected_model_failures(error: Exception) -> None:
    service = ConsultationIntakeService(model_resolver=lambda _config: object())  # type: ignore[arg-type]
    with patch(
        "src.services.consultation_intake_service.create_consultation_intake_agent",
        return_value=_FailingAgent(error),
    ):
        result = await service.assess(
            latest_user_message="我右臀疼了两周，久坐更明显",
            profile={},
            body_state={},
            config=get_default_consultation_configuration(),
        )

    assert result.turn_kind == "symptom_report"
    assert result.symptoms
    assert result.symptoms[0].body_part == "右臀"


@pytest.mark.asyncio
async def test_intake_does_not_hide_unexpected_runtime_errors() -> None:
    service = ConsultationIntakeService(model_resolver=lambda _config: object())  # type: ignore[arg-type]
    with patch(
        "src.services.consultation_intake_service.create_consultation_intake_agent",
        return_value=_FailingAgent(RuntimeError("programming defect")),
    ):
        with pytest.raises(RuntimeError, match="programming defect"):
            await service.assess(
                latest_user_message="我右臀疼了两周",
                profile={},
                body_state={},
                config=get_default_consultation_configuration(),
            )
