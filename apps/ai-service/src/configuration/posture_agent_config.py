"""Immutable, repository-versioned Posture Agent configuration manifests.

Posture owns two behavior classes that must move together under one immutable
configuration identity:

- VLM behavior (prompt/schema/model/governance/generation); and
- deterministic geometric perception (engine/model artifact/threshold contract).

Historical v1 has no pinned geometry mechanism. Current v2 pins that mechanism
explicitly so the same Posture configuration cannot silently mean VLM-only in
one environment and VLM + MediaPipe geometry in another.
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
DEFAULT_MANIFEST_PATH = CONFIG_ROOT / "posture-v2.yaml"


class PostureGenerationConfig(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    temperature: float = Field(ge=0.0, le=2.0)
    max_tokens: int = Field(gt=0)


class PostureGeometryMechanismConfig(BaseModel):
    """Pinned non-LLM mechanism identity for authoritative geometric metrics."""

    model_config = ConfigDict(frozen=True, extra="forbid")
    required: Literal[True] = True
    mechanism_revision: str = Field(min_length=1)
    engine: Literal["mediapipe-tasks"]
    engine_version: str = Field(min_length=1)
    model_uri: str = Field(min_length=1)
    model_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    threshold_revision: str = Field(min_length=1)
    threshold_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class PostureAgentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["posture"]
    manifest_revision: str = Field(min_length=1)
    logical_model: str = Field(min_length=1)
    model_group_revision: str = Field(min_length=1)
    prompt_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    tool_policy_revision: str = Field(min_length=1)
    governance_policy_revision: str = Field(min_length=1)
    decision_policy_revision: str = Field(min_length=1)
    geometry_mechanism: PostureGeometryMechanismConfig | None = None
    generation: PostureGenerationConfig

    def canonical_behavior_json(self) -> str:
        # exclude_none keeps the historical posture-v1 fingerprint byte-for-byte
        # compatible even though the schema now understands v2 geometry identity.
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
        return f"posture-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, str]:
        fields = self.model_dump(
            mode="json",
            exclude={"generation", "geometry_mechanism"},
            exclude_none=True,
        )
        result = {"id": self.configuration_id, **{key: str(value) for key, value in fields.items()}}
        if self.geometry_mechanism is not None:
            result["geometry_mechanism_revision"] = self.geometry_mechanism.mechanism_revision
        return result


def _load_yaml(path: Path) -> dict[str, object]:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(raw, dict):
        raise ValueError(f"configuration file must contain a mapping: {path}")
    return raw


def load_manifest(path: Path) -> PostureAgentManifest:
    return PostureAgentManifest.model_validate(_load_yaml(path))


@lru_cache(maxsize=1)
def get_default_posture_configuration() -> PostureAgentManifest:
    return load_manifest(DEFAULT_MANIFEST_PATH)


@lru_cache(maxsize=32)
def get_posture_configuration(configuration_id: str) -> PostureAgentManifest:
    for path in sorted(CONFIG_ROOT.glob("posture-*.yaml")):
        config = load_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown Posture configuration_id: {configuration_id}")


def known_posture_configuration_ids() -> list[str]:
    return sorted(
        load_manifest(path).configuration_id for path in CONFIG_ROOT.glob("posture-*.yaml")
    )
