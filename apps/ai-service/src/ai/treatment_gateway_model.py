"""Immutable-config-aware Treatment model boundary for the LiteLLM gateway."""

from __future__ import annotations

from pydantic_ai.models import Model

from ..configuration.treatment_agent_config import TreatmentAgentManifest
from .gateway import get_gateway_model

TREATMENT_MODEL_GROUP_REVISION = "treatment-model-group-v1"


def get_treatment_runtime_model(config: TreatmentAgentManifest) -> Model:
    if config.model_group_revision != TREATMENT_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Treatment model group revision: {config.model_group_revision}"
        )
    return get_gateway_model(config.logical_model)


def treatment_model_settings(config: TreatmentAgentManifest) -> dict[str, object]:
    if config.model_group_revision != TREATMENT_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Treatment model group revision: {config.model_group_revision}"
        )
    return {
        "temperature": config.generation.temperature,
        "max_tokens": config.generation.max_tokens,
    }
