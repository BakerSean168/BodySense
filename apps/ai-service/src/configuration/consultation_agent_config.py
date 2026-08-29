"""Immutable, repository-versioned Consultation Agent configuration manifests.

Consultation is a multi-turn LangGraph runtime (not a single-shot PydanticAI
Agent), but it still gets the same immutable execution identity: a
repository-versioned manifest whose canonical behavior JSON is fingerprinted,
so the Agent configuration and its execution provenance are exactly
reproducible and auditable.
"""

from __future__ import annotations

import hashlib
import json
from functools import lru_cache
from pathlib import Path
from typing import Any, Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field

SERVICE_ROOT = Path(__file__).resolve().parents[2]
CONFIG_ROOT = SERVICE_ROOT / "config" / "agents"
DEFAULT_MANIFEST_PATH = CONFIG_ROOT / "consultation-v2.yaml"


class ConsultationGenerationConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    temperature: float = Field(ge=0.0, le=2.0)
    max_tokens: int = Field(gt=0)


class ConsultationIntakeConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    policy_revision: str = Field(min_length=1)
    generation: ConsultationGenerationConfig


class ConsultationAgentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["consultation"]
    manifest_revision: str = Field(min_length=1)
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    tool_policy_revision: str = Field(min_length=1)
    governance_policy_revision: str = Field(min_length=1)
    decision_policy_revision: str = Field(min_length=1)
    generation: ConsultationGenerationConfig
    intake: ConsultationIntakeConfig | None = None

    def canonical_behavior_json(self) -> str:
        return json.dumps(
            self.model_dump(
                mode="json",
                exclude={"manifest_revision"},
                exclude_none=True,
            ),
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )

    @property
    def fingerprint(self) -> str:
        return hashlib.sha256(self.canonical_behavior_json().encode()).hexdigest()

    @property
    def configuration_id(self) -> str:
        return f"consult-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, Any]:
        fields = self.model_dump(
            mode="json",
            exclude={"generation"},
            exclude_none=True,
        )
        return {"id": self.configuration_id, **fields}


def _load_yaml(path: Path) -> dict[str, object]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"configuration file must contain a mapping: {path}")
    return raw


def load_manifest(path: Path) -> ConsultationAgentManifest:
    return ConsultationAgentManifest.model_validate(_load_yaml(path))


@lru_cache(maxsize=1)
def get_default_consultation_configuration() -> ConsultationAgentManifest:
    return load_manifest(DEFAULT_MANIFEST_PATH)


@lru_cache(maxsize=32)
def get_consultation_configuration(configuration_id: str) -> ConsultationAgentManifest:
    for path in sorted(CONFIG_ROOT.glob("consultation-*.yaml")):
        config = load_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Consultation configuration_id: {configuration_id}")


def known_consultation_configuration_ids() -> list[str]:
    return sorted(
        load_manifest(path).configuration_id for path in CONFIG_ROOT.glob("consultation-*.yaml")
    )
