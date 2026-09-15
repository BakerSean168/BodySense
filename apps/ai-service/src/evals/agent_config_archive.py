"""Offline-only access to retired Agent configuration manifests.

Production runtime loaders intentionally scan only ``config/agents``. Historical
manifests live under the eval corpus so qualification/promotion evidence remains
reproducible without making a retired configuration a serving target again.
"""

from __future__ import annotations

from pathlib import Path

from src.configuration.diagnosis_agent_config import (
    DiagnosisAgentManifest,
    get_diagnosis_configuration,
)
from src.configuration.diagnosis_agent_config import (
    load_manifest as load_diagnosis_manifest,
)
from src.configuration.treatment_agent_config import (
    TreatmentAgentManifest,
    get_treatment_configuration,
)
from src.configuration.treatment_agent_config import (
    load_manifest as load_treatment_manifest,
)

SERVICE_ROOT = Path(__file__).resolve().parents[2]
ARCHIVED_AGENT_CONFIG_ROOT = SERVICE_ROOT / "data" / "evals" / "agent-configurations"


def _resolve_archived_diagnosis(configuration_id: str) -> DiagnosisAgentManifest:
    for path in sorted(ARCHIVED_AGENT_CONFIG_ROOT.glob("diagnosis-*.yaml")):
        config = load_diagnosis_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Diagnosis evaluation configuration_id: {configuration_id}")


def get_diagnosis_evaluation_configuration(configuration_id: str) -> DiagnosisAgentManifest:
    try:
        return get_diagnosis_configuration(configuration_id)
    except ValueError:
        return _resolve_archived_diagnosis(configuration_id)


def _resolve_archived_treatment(configuration_id: str) -> TreatmentAgentManifest:
    for path in sorted(ARCHIVED_AGENT_CONFIG_ROOT.glob("treatment-*.yaml")):
        config = load_treatment_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Treatment evaluation configuration_id: {configuration_id}")


def get_treatment_evaluation_configuration(configuration_id: str) -> TreatmentAgentManifest:
    try:
        return get_treatment_configuration(configuration_id)
    except ValueError:
        return _resolve_archived_treatment(configuration_id)
