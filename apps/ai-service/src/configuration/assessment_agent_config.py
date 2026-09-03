"""Immutable, repository-versioned Assessment Agent configuration manifests.

Assessment is a derived report (not a second health truth and not a Treatment
system). It consumes Profile, Posture analysis and current BodyState, and emits
Observation candidates plus information gaps that Go projects into BodyState.
This module gives that role the same immutable execution identity as Diagnosis
and Treatment: a repository-versioned manifest whose canonical behavior JSON is
fingerprinted, so the Agent configuration and its execution provenance are
exactly reproducible and auditable.
"""

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
DEFAULT_MANIFEST_PATH = CONFIG_ROOT / "assessment-v5.yaml"


class AssessmentGenerationConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    temperature: float = Field(ge=0.0, le=2.0)
    max_tokens: int = Field(gt=0)


class AssessmentAgentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["assessment"]
    manifest_revision: str = Field(min_length=1)
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    tool_policy_revision: str = Field(min_length=1)
    evidence_policy_revision: str = Field(min_length=1)
    governance_policy_revision: str = Field(min_length=1)
    decision_policy_revision: str = Field(min_length=1)
    generation: AssessmentGenerationConfig

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
        return f"assess-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, str]:
        fields = self.model_dump(mode="json", exclude={"generation"})
        return {"id": self.configuration_id, **{key: str(value) for key, value in fields.items()}}


def _load_yaml(path: Path) -> dict[str, object]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"configuration file must contain a mapping: {path}")
    return raw


def load_manifest(path: Path) -> AssessmentAgentManifest:
    return AssessmentAgentManifest.model_validate(_load_yaml(path))


@lru_cache(maxsize=1)
def get_default_assessment_configuration() -> AssessmentAgentManifest:
    return load_manifest(DEFAULT_MANIFEST_PATH)


@lru_cache(maxsize=32)
def get_assessment_configuration(configuration_id: str) -> AssessmentAgentManifest:
    for path in sorted(CONFIG_ROOT.glob("assessment-*.yaml")):
        config = load_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Assessment configuration_id: {configuration_id}")


def known_assessment_configuration_ids() -> list[str]:
    return sorted(
        load_manifest(path).configuration_id for path in CONFIG_ROOT.glob("assessment-*.yaml")
    )
