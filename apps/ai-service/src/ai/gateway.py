"""Single internal LiteLLM gateway transport and logical route registry.

Application code chooses a BodySense logical use-case. Physical providers,
credentials, retry, and fallback are exclusively gateway responsibilities.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from functools import lru_cache

from pydantic_ai.models import Model
from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.openai import OpenAIProvider

from .errors import GatewayUnavailableError

_DEFAULT_GATEWAY_BASE_URL = "http://localhost:4000/v1"

CONSULTATION_ROUTE = "consultation.reply"
ASSESSMENT_ROUTE = "assessment.generate"
TREATMENT_ROUTE = "treatment.proposal"
KNOWLEDGE_CURATOR_ROUTE = "knowledge.curate"
KNOWLEDGE_SPLITTER_ROUTE = "knowledge.split"
TITLE_ROUTE = "conversation.title"
POSTURE_ROUTE = "posture.analyze"


@dataclass(frozen=True)
class GatewayRoute:
    logical_model: str
    temperature: float
    max_tokens: int
    response_format: str | None = None


_GATEWAY_ROUTES: dict[str, GatewayRoute] = {
    CONSULTATION_ROUTE: GatewayRoute("bodysense-consultation", 0.7, 2048),
    ASSESSMENT_ROUTE: GatewayRoute("bodysense-structured", 0.3, 2048),
    TREATMENT_ROUTE: GatewayRoute("bodysense-structured", 0.3, 2048),
    KNOWLEDGE_CURATOR_ROUTE: GatewayRoute("bodysense-structured", 0.3, 2048, "json_object"),
    KNOWLEDGE_SPLITTER_ROUTE: GatewayRoute("bodysense-structured", 0.3, 2048, "json_object"),
    TITLE_ROUTE: GatewayRoute("bodysense-text", 0.3, 100),
    POSTURE_ROUTE: GatewayRoute("bodysense-posture", 0.2, 1500, "json_object"),
}


def gateway_route(use_case: str) -> GatewayRoute:
    route = _GATEWAY_ROUTES.get(use_case)
    if route is None:
        raise GatewayUnavailableError(f"No LiteLLM logical route for {use_case}")
    return route


def gateway_credentials() -> tuple[str, str]:
    base_url = os.getenv("LITELLM_BASE_URL", _DEFAULT_GATEWAY_BASE_URL).strip()
    api_key = os.getenv("LITELLM_API_KEY", "").strip()
    if not base_url:
        raise GatewayUnavailableError("LITELLM_BASE_URL is required")
    if not api_key:
        raise GatewayUnavailableError("LITELLM_API_KEY is required")
    return base_url, api_key


@lru_cache(maxsize=32)
def get_gateway_model(logical_model: str) -> Model:
    """Return one logical model backed only by the internal LiteLLM gateway."""

    if not logical_model.strip():
        raise GatewayUnavailableError("LiteLLM logical model is required")
    base_url, api_key = gateway_credentials()
    provider = OpenAIProvider(base_url=base_url, api_key=api_key)
    return OpenAIChatModel(logical_model, provider=provider)


@lru_cache(maxsize=16)
def get_gateway_pydantic_model(use_case: str) -> Model:
    """Resolve a generic business route to its logical gateway model."""

    return get_gateway_model(gateway_route(use_case).logical_model)


def gateway_model_settings(use_case: str) -> dict[str, object]:
    route = gateway_route(use_case)
    return {"temperature": route.temperature, "max_tokens": route.max_tokens}
