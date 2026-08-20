"""Diagnosis-only PydanticAI boundary for the standalone LiteLLM gateway.

BodySense chooses a logical Agent model here. Physical provider construction,
retry, and fallback belong exclusively to the gateway.
"""

from __future__ import annotations

from functools import lru_cache

from pydantic_ai.models import Model
from pydantic_ai.models.openai import OpenAIChatModel
from pydantic_ai.providers.openai import OpenAIProvider

from ..configuration.diagnosis_agent_config import DiagnosisAgentManifest
from .gateway import gateway_credentials

DIAGNOSIS_LOGICAL_MODEL = "bodysense-diagnosis"
DIAGNOSIS_MODEL_GROUP_REVISION = "diagnosis-model-group-v1"


@lru_cache(maxsize=1)
def get_diagnosis_gateway_model(logical_model: str = DIAGNOSIS_LOGICAL_MODEL) -> OpenAIChatModel:
    """Return the logical Diagnosis model exposed by the internal gateway."""

    base_url, api_key = gateway_credentials()
    provider = OpenAIProvider(base_url=base_url, api_key=api_key)
    return OpenAIChatModel(logical_model, provider=provider)


def get_diagnosis_runtime_model(config: DiagnosisAgentManifest) -> Model:
    """Resolve Diagnosis exclusively through its logical LiteLLM model group."""

    if config.model_group_revision != DIAGNOSIS_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Diagnosis model group revision: {config.model_group_revision}"
        )
    return get_diagnosis_gateway_model(config.logical_model)


def diagnosis_model_settings(config: DiagnosisAgentManifest) -> dict[str, object]:
    """Resolve generation settings after validating the model-group revision."""

    if config.model_group_revision != DIAGNOSIS_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Diagnosis model group revision: {config.model_group_revision}"
        )
    generation = config.generation
    return {"temperature": generation.temperature, "max_tokens": generation.max_tokens}
