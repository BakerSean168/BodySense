"""Shared immutable Agent configuration loader for single-shot Python Agents."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field

SERVICE_ROOT = Path(__file__).resolve().parents[2]
CONFIG_ROOT = SERVICE_ROOT / "config" / "agents"


class SingleShotGenerationConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    temperature: float = Field(ge=0.0, le=2.0)
    max_tokens: int = Field(gt=0)


class SingleShotAgentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["knowledge_curator", "knowledge_splitter"]
    manifest_revision: str = Field(min_length=1)
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    tool_policy_revision: str = Field(min_length=1)
    governance_policy_revision: str = Field(min_length=1)
    decision_policy_revision: str = Field(min_length=1)
    generation: SingleShotGenerationConfig

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
        prefix = "knowledge-curator" if self.role == "knowledge_curator" else "knowledge-splitter"
        return f"{prefix}-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, str]:
        fields = self.model_dump(mode="json", exclude={"generation"})
        return {"id": self.configuration_id, **{key: str(value) for key, value in fields.items()}}


def _load_yaml(path: Path) -> dict[str, object]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"configuration file must contain a mapping: {path}")
    return raw


def load_manifest(path: Path) -> SingleShotAgentManifest:
    return SingleShotAgentManifest.model_validate(_load_yaml(path))


def get_knowledge_curator_configuration(configuration_id: str | None = None) -> SingleShotAgentManifest:
    if configuration_id:
        for path in sorted(CONFIG_ROOT.glob("knowledge-curator-*.yaml")):
            config = load_manifest(path)
            if config.configuration_id == configuration_id:
                return config
        raise ValueError(f"unknown Knowledge Curator configuration_id: {configuration_id}")
    return load_manifest(CONFIG_ROOT / "knowledge-curator-v1.yaml")


def get_knowledge_splitter_configuration(configuration_id: str | None = None) -> SingleShotAgentManifest:
    if configuration_id:
        for path in sorted(CONFIG_ROOT.glob("knowledge-splitter-*.yaml")):
            config = load_manifest(path)
            if config.configuration_id == configuration_id:
                return config
        raise ValueError(f"unknown Knowledge Splitter configuration_id: {configuration_id}")
    return load_manifest(CONFIG_ROOT / "knowledge-splitter-v1.yaml")
