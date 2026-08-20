"""Immutable-config-aware Assessment model boundary for the LiteLLM gateway."""

from __future__ import annotations

from pydantic_ai.models import Model

from ..configuration.assessment_agent_config import AssessmentAgentManifest
from .gateway import get_gateway_model

ASSESSMENT_MODEL_GROUP_REVISION = "assessment-model-group-v1"


def get_assessment_runtime_model(config: AssessmentAgentManifest) -> Model:
    if config.model_group_revision != ASSESSMENT_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Assessment model group revision: {config.model_group_revision}"
        )
    return get_gateway_model(config.logical_model)


def assessment_model_settings(config: AssessmentAgentManifest) -> dict[str, object]:
    if config.model_group_revision != ASSESSMENT_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Assessment model group revision: {config.model_group_revision}"
        )
    return {
        "temperature": config.generation.temperature,
        "max_tokens": config.generation.max_tokens,
    }
