"""Immutable-config-aware Posture model boundary for the LiteLLM gateway."""

from __future__ import annotations

from ..configuration.posture_agent_config import PostureAgentManifest
from .gateway import get_gateway_model

POSTURE_MODEL_GROUP_REVISION = "posture-model-group-v1"


def get_posture_runtime_model(config: PostureAgentManifest):
    if config.model_group_revision != POSTURE_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Posture model group revision: {config.model_group_revision}"
        )
    return get_gateway_model(config.logical_model)


def posture_model_settings(config: PostureAgentManifest) -> dict[str, object]:
    if config.model_group_revision != POSTURE_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Posture model group revision: {config.model_group_revision}"
        )
    return {
        "temperature": config.generation.temperature,
        "max_tokens": config.generation.max_tokens,
    }
