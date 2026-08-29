"""Model boundary for the Consultation typed intake preflight."""

from __future__ import annotations

from pydantic_ai.models import Model
from pydantic_ai.settings import ModelSettings

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from .gateway import get_gateway_model

CONSULTATION_INTAKE_MODEL_GROUP_REVISION = "consultation-intake-model-group-v1"


def get_consultation_intake_runtime_model(config: ConsultationAgentManifest) -> Model:
    intake = config.intake
    if intake is None:
        raise ValueError("Consultation configuration has no intake preflight")
    if intake.model_group_revision != CONSULTATION_INTAKE_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Consultation intake model group revision: {intake.model_group_revision}"
        )
    return get_gateway_model(intake.logical_model)


def consultation_intake_model_settings(config: ConsultationAgentManifest) -> ModelSettings:
    intake = config.intake
    if intake is None:
        raise ValueError("Consultation configuration has no intake preflight")
    if intake.model_group_revision != CONSULTATION_INTAKE_MODEL_GROUP_REVISION:
        raise ValueError(
            f"unsupported Consultation intake model group revision: {intake.model_group_revision}"
        )
    return {
        "temperature": intake.generation.temperature,
        "max_tokens": intake.generation.max_tokens,
    }
