from pathlib import Path

import pytest
import yaml
from pydantic import ValidationError

from src.ai.errors import GatewayUnavailableError
from src.ai.gateway import (
    ASSESSMENT_ROUTE,
    CONSULTATION_ROUTE,
    KNOWLEDGE_CURATOR_ROUTE,
    KNOWLEDGE_SPLITTER_ROUTE,
    POSTURE_ROUTE,
    TITLE_ROUTE,
    TREATMENT_ROUTE,
    gateway_model_settings,
    gateway_route,
    get_gateway_pydantic_model,
)

ROOT = Path(__file__).resolve().parents[4]


def test_business_routes_resolve_only_to_gateway_logical_models() -> None:
    expected = {
        CONSULTATION_ROUTE: "bodysense-consultation",
        ASSESSMENT_ROUTE: "bodysense-structured",
        TREATMENT_ROUTE: "bodysense-structured",
        KNOWLEDGE_CURATOR_ROUTE: "bodysense-structured",
        KNOWLEDGE_SPLITTER_ROUTE: "bodysense-structured",
        TITLE_ROUTE: "bodysense-text",
        POSTURE_ROUTE: "bodysense-posture",
    }
    assert {route: gateway_route(route).logical_model for route in expected} == expected
    assert gateway_model_settings(ASSESSMENT_ROUTE) == {"temperature": 0.3, "max_tokens": 2048}


def test_gateway_pydantic_model_uses_internal_gateway_only(monkeypatch) -> None:
    monkeypatch.setenv("LITELLM_BASE_URL", "http://gateway.internal:4000/v1")
    monkeypatch.setenv("LITELLM_API_KEY", "gateway-only-secret")
    monkeypatch.setenv("MIMO_API_KEY", "must-not-be-read-by-ai-service")
    monkeypatch.setenv("OPENROUTER_API_KEY", "must-not-be-read-by-ai-service")
    get_gateway_pydantic_model.cache_clear()

    model = get_gateway_pydantic_model(TREATMENT_ROUTE)

    assert model.model_name == "bodysense-structured"
    assert str(model.provider.base_url) == "http://gateway.internal:4000/v1/"


def test_unknown_business_route_fails_closed() -> None:
    with pytest.raises(GatewayUnavailableError, match="No LiteLLM logical route"):
        gateway_route("physical-provider/model")


def test_retired_router_artifacts_do_not_exist() -> None:
    for retired in (
        "apps/ai-service/src/ai/config.py",
        "apps/ai-service/src/ai/router.py",
        "apps/ai-service/src/ai/pydantic_model.py",
        "apps/ai-service/src/config/models.yaml",
        "apps/ai-service/src/ai/diagnosis_model_boundary.py",
    ):
        assert not (ROOT / retired).exists()


def test_ai_service_compose_receives_gateway_credentials_not_llm_provider_secrets() -> None:
    retired_llm_env = {
        "LLM_PROVIDER",
        "LLM_API_KEY",
        "LLM_MODEL",
        "LLM_BASE_URL",
        "OPENROUTER_API_KEY",
        "MIMO_API_KEY",
        "MIMO_BASE_URL",
    }
    for relative in (
        "docker/docker-compose.yml",
        "docker/docker-compose.prod.yml",
    ):
        config = yaml.safe_load((ROOT / relative).read_text(encoding="utf-8"))
        ai_env = config["services"]["ai-service"]["environment"]
        gateway_env = config["services"]["litellm-gateway"]["environment"]
        assert "LITELLM_BASE_URL" in ai_env
        assert "LITELLM_API_KEY" in ai_env
        assert retired_llm_env.isdisjoint(ai_env)
        assert "MIMO_API_KEY" in gateway_env
        assert "OPENROUTER_API_KEY" in gateway_env


def test_internal_typed_agent_http_contracts_reject_retired_model_route_intent() -> None:
    from src.api.routes.assessment import AssessmentRequest
    from src.api.routes.treatment import TreatmentRecommendationRequest

    with pytest.raises(ValidationError, match="use_case"):
        AssessmentRequest.model_validate({"profile": {}, "use_case": "llm.json"})
    with pytest.raises(ValidationError, match="use_case"):
        TreatmentRecommendationRequest.model_validate(
            {
                "body_state_revision": 1,
                "body_state": {},
                "diagnosis_analysis": {},
                "use_case": "llm.json",
            }
        )


@pytest.mark.asyncio
async def test_ai_service_honors_manifest_pinned_logical_model() -> None:
    from types import SimpleNamespace

    from src.ai.service import AIService
    from src.ai.types import AiRequest, ChatMessage

    class _Completions:
        def __init__(self) -> None:
            self.kwargs: dict[str, object] | None = None

        async def create(self, **kwargs):
            self.kwargs = kwargs
            return SimpleNamespace(
                choices=[
                    SimpleNamespace(
                        message=SimpleNamespace(content="ok", tool_calls=None),
                        finish_reason="stop",
                    )
                ],
                usage=None,
                model="manifest-model",
            )

    completions = _Completions()
    service = object.__new__(AIService)
    service._client = SimpleNamespace(chat=SimpleNamespace(completions=completions))

    await service.generate(
        AiRequest(
            use_case=TITLE_ROUTE,
            messages=[ChatMessage(role="user", content="hello")],
            logical_model="manifest-model",
            model_settings={"temperature": 0.0, "max_tokens": 12},
            temperature=0.0,
            max_tokens=12,
        )
    )

    assert completions.kwargs is not None
    assert completions.kwargs["model"] == "manifest-model"


@pytest.mark.asyncio
async def test_ai_service_stream_honors_manifest_pinned_logical_model() -> None:
    from types import SimpleNamespace

    from src.ai.service import AIService
    from src.ai.types import AiRequest, ChatMessage

    class _EmptyStream:
        def __aiter__(self):
            return self

        async def __anext__(self):
            raise StopAsyncIteration

    class _Completions:
        def __init__(self) -> None:
            self.kwargs: dict[str, object] | None = None

        async def create(self, **kwargs):
            self.kwargs = kwargs
            return _EmptyStream()

    completions = _Completions()
    service = object.__new__(AIService)
    service._client = SimpleNamespace(chat=SimpleNamespace(completions=completions))

    events = [
        event
        async for event in service.generate_stream(
            AiRequest(
                use_case=POSTURE_ROUTE,
                messages=[ChatMessage(role="user", content="hello")],
                logical_model="manifest-vision-model",
                model_settings={"temperature": 0.0, "max_tokens": 12},
                temperature=0.0,
                max_tokens=12,
            )
        )
    ]

    assert events == []
    assert completions.kwargs is not None
    assert completions.kwargs["model"] == "manifest-vision-model"


def test_go_owned_internal_http_boundaries_require_configuration_identity() -> None:
    from src.api.routes.assessment import AssessmentRequest
    from src.api.routes.runtime import StartTurnRequest
    from src.api.routes.title import TitleGenerateRequest

    with pytest.raises(ValidationError, match="configuration_id"):
        AssessmentRequest.model_validate({"profile": {}})
    with pytest.raises(ValidationError, match="configuration_id"):
        TitleGenerateRequest.model_validate({"messages": []})
    with pytest.raises(ValidationError, match="configuration_id"):
        StartTurnRequest.model_validate(
            {
                "run_id": "run-1",
                "conversation_id": "conversation-1",
                "user_id": "user-1",
                "input": {"type": "user_message", "text": "hello"},
            }
        )
