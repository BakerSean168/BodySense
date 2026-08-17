"""PydanticAI model composition backed by BodySense route configuration.

This is the deliberate replacement for routing Diagnosis/Treatment through a raw
``AIService.generate -> string`` boundary. The same models.yaml candidate order is
preserved, while PydanticAI owns structured output and tool execution.
"""

from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path

from pydantic_ai.models import Model
from pydantic_ai.models.fallback import FallbackModel
from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.openai import OpenAIProvider

from .config import load_config
from .errors import NoAvailableProviderError

_DEFAULT_CONFIG_PATH = Path(__file__).parent.parent / "config" / "models.yaml"


@lru_cache(maxsize=16)
def get_pydantic_model(use_case: str = "llm.json") -> Model:
    """Build and cache the configured PydanticAI model/fallback chain."""

    config_path = Path(os.environ.get("AI_CONFIG_PATH", _DEFAULT_CONFIG_PATH))
    config = load_config(config_path)
    route = config.routes.get(use_case)
    if route is None:
        raise NoAvailableProviderError(f"No route for {use_case}")

    models: list[Model] = []
    for candidate in route.candidates:
        provider_config = config.providers.get(candidate.provider)
        if provider_config is None:
            continue
        if provider_config.type != "openai-compatible":
            continue
        provider = OpenAIProvider(
            base_url=provider_config.base_url,
            api_key=provider_config.api_key,
        )
        models.append(OpenAIChatModel(candidate.model, provider=provider))

    if not models:
        raise NoAvailableProviderError(f"No configured PydanticAI model for {use_case}")
    if len(models) == 1:
        return models[0]
    return FallbackModel(models[0], *models[1:])


def route_model_settings(use_case: str = "llm.json") -> dict[str, object]:
    """Return only provider-neutral settings understood by PydanticAI."""

    config_path = Path(os.environ.get("AI_CONFIG_PATH", _DEFAULT_CONFIG_PATH))
    config = load_config(config_path)
    route = config.routes.get(use_case)
    if route is None:
        raise NoAvailableProviderError(f"No route for {use_case}")
    settings: dict[str, object] = {}
    if route.defaults.temperature is not None:
        settings["temperature"] = route.defaults.temperature
    if route.defaults.max_tokens is not None:
        settings["max_tokens"] = route.defaults.max_tokens
    return settings
