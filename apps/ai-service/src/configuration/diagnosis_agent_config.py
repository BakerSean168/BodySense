"""Immutable, repository-versioned Diagnosis Agent configuration manifests."""

from __future__ import annotations

import hashlib
import json
from functools import lru_cache
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field

SERVICE_ROOT = Path(__file__).resolve().parents[2]
CONFIG_ROOT = SERVICE_ROOT / "config" / "agents"
DEFAULT_MANIFEST_PATH = CONFIG_ROOT / "diagnosis-v1.yaml"


class GenerationConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    temperature: float = Field(ge=0.0, le=2.0)
    max_tokens: int = Field(gt=0)


class DiagnosisAgentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["diagnosis"]
    manifest_revision: str = Field(min_length=1)
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    tool_policy_revision: str = Field(min_length=1)
    evidence_policy_revision: str = Field(min_length=1)
    governance_policy_revision: str = Field(min_length=1)
    decision_policy_revision: str = Field(min_length=1)
    generation: GenerationConfig

    def canonical_behavior_json(self) -> str:
        return json.dumps(
            self.model_dump(mode="json", exclude={"manifest_revision"}),
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )

    @property
    def fingerprint(self) -> str:
        return hashlib.sha256(self.canonical_behavior_json().encode()).hexdigest()

    @property
    def configuration_id(self) -> str:
        return f"diag-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, str]:
        fields = self.model_dump(mode="json", exclude={"generation"})
        return {"id": self.configuration_id, **{k: str(v) for k, v in fields.items()}}


def _load_yaml(path: Path) -> dict[str, object]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"configuration file must contain a mapping: {path}")
    return raw


def load_manifest(path: Path) -> DiagnosisAgentManifest:
    return DiagnosisAgentManifest.model_validate(_load_yaml(path))


@lru_cache(maxsize=1)
def get_default_diagnosis_configuration() -> DiagnosisAgentManifest:
    """Repository default used by deterministic evals and bootstrapping only."""
    return load_manifest(DEFAULT_MANIFEST_PATH)


@lru_cache(maxsize=32)
def get_diagnosis_configuration(configuration_id: str) -> DiagnosisAgentManifest:
    """Resolve exactly the immutable configuration selected by the Go control plane."""
    for path in sorted(CONFIG_ROOT.glob("diagnosis-*.yaml")):
        config = load_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Diagnosis configuration_id: {configuration_id}")
