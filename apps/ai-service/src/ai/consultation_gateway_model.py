"""Immutable-config-aware Consultation model boundary for the LiteLLM gateway.

Consultation is a multi-turn LangGraph runtime (not a single-shot PydanticAI
Agent), but it still resolves its exact logical model + generation settings
from the immutable manifest rather than from the use_case route lookup.
"""

from __future__ import annotations

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from .gateway import get_gateway_model

CONSULTATION_MODEL_GROUP_REVISION = "consultation-model-group-v1"


def get_consultation_runtime_model(config: ConsultationAgentManifest):
    if config.model_group_revision != CONSULTATION_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Consultation model group revision: {config.model_group_revision}"
        )
    return get_gateway_model(config.logical_model)


def consultation_model_settings(config: ConsultationAgentManifest) -> dict[str, object]:
    if config.model_group_revision != CONSULTATION_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Consultation model group revision: {config.model_group_revision}"
        )
    return {
        "temperature": config.generation.temperature,
        "max_tokens": config.generation.max_tokens,
    }
