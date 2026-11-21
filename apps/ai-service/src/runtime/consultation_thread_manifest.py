"""Resolve the immutable Consultation Agent configuration for one turn."""

from __future__ import annotations

from typing import Any

from ..configuration.consultation_agent_config import (
    ConsultationAgentManifest,
    get_consultation_configuration,
    get_default_consultation_configuration,
)


def get_consultation_manifest(configuration_id: str | None = None):
    """Resolve the exact immutable Consultation Agent configuration."""
    if configuration_id:
        return get_consultation_configuration(configuration_id)
    return get_default_consultation_configuration()


def _checkpoint_manifest_configuration_id(value: Any) -> str:
    if isinstance(value, ConsultationAgentManifest):
        return value.configuration_id
    if isinstance(value, dict):
        return ConsultationAgentManifest.model_validate(value).configuration_id
    raise ValueError("Consultation checkpoint is missing an immutable configuration manifest")
