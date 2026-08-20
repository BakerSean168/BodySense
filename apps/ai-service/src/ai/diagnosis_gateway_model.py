"""Diagnosis-only PydanticAI boundary for the standalone LiteLLM gateway.

BodySense chooses a logical Agent model here. Physical provider construction,
retry, and fallback belong exclusively to the gateway.
"""

from __future__ import annotations

import os
from functools import lru_cache

from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.openai import OpenAIProvider

from .errors import NoAvailableProviderError

DIAGNOSIS_LOGICAL_MODEL = "bodysense-diagnosis"
_DEFAULT_GATEWAY_BASE_URL = "http://localhost:4000/v1"


@lru_cache(maxsize=1)
def get_diagnosis_gateway_model() -> OpenAIChatModel:
    """Return the logical Diagnosis model exposed by the internal gateway."""

    base_url = os.getenv("LITELLM_BASE_URL", _DEFAULT_GATEWAY_BASE_URL).strip()
    api_key = os.getenv("LITELLM_API_KEY", "").strip()
    if not base_url:
        raise NoAvailableProviderError("LITELLM_BASE_URL is required for Diagnosis")
    if not api_key:
        raise NoAvailableProviderError("LITELLM_API_KEY is required for Diagnosis")

    provider = OpenAIProvider(base_url=base_url, api_key=api_key)
    return OpenAIChatModel(DIAGNOSIS_LOGICAL_MODEL, provider=provider)


def diagnosis_model_settings() -> dict[str, object]:
    """Preserve the pre-cutover Diagnosis generation settings until Phase 3 manifests them."""

    return {"temperature": 0.3, "max_tokens": 2048}
