"""Immutable repository configuration for health-document extraction."""

from __future__ import annotations

import hashlib
import json
from functools import lru_cache
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, ConfigDict, Field, model_validator

SERVICE_ROOT = Path(__file__).resolve().parents[2]
CONFIG_ROOT = SERVICE_ROOT / "config" / "document-extraction"
DEFAULT_MANIFEST_PATH = CONFIG_ROOT / "health-document-v20.yaml"


class HealthDocumentModelArtifact(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["det", "rec", "cls"]
    filename: str = Field(min_length=1)
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class HealthDocumentVerifierArtifact(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    language: str = Field(min_length=1)
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class HealthDocumentManifest(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    role: Literal["health_document"]
    manifest_revision: str = Field(min_length=1)
    mechanism_revision: str = Field(min_length=1)
    output_schema_revision: str = Field(min_length=1)
    execution_topology_revision: Literal[
        "per-document-subprocess-v1",
        "primary-then-verifier-subprocess-v1",
    ]
    pdf_strategy_revision: str = Field(min_length=1)
    native_text_engine: Literal["pymupdf"]
    native_text_engine_version: str = Field(min_length=1)
    native_text_quality_policy_revision: str = Field(min_length=1)
    native_text_quality_policy_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    ocr_engine: Literal["rapidocr"]
    ocr_engine_version: str = Field(min_length=1)
    runtime_engine: Literal["onnxruntime"]
    runtime_version: str = Field(min_length=1)
    model_family: Literal["PP-OCRv6"]
    model_type: Literal["small"]
    model_artifacts: list[HealthDocumentModelArtifact] = Field(min_length=3, max_length=3)
    pdf_raster_dpi: int = Field(ge=72, le=600)
    detector_limit_type: Literal["max"]
    detector_limit_side_len: int = Field(ge=128, le=4096)
    global_max_side_len: int | None = Field(default=None, ge=128, le=4096)
    rec_batch_num: int | None = Field(default=None, ge=1, le=64)
    cls_batch_num: int | None = Field(default=None, ge=1, le=64)
    ort_intra_op_num_threads: int | None = Field(default=None, ge=1, le=64)
    ort_inter_op_num_threads: int | None = Field(default=None, ge=1, le=64)
    indicator_parser_revision: str = Field(min_length=1)
    indicator_parser_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    admissibility_policy_revision: str = Field(min_length=1)
    admissibility_policy_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    engine_adapter_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    worker_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    verification_revision: str | None = None
    verifier_engine: Literal["tesseract"] | None = None
    verifier_engine_version: str | None = None
    verifier_languages: list[str] | None = None
    verifier_language_artifacts: list[HealthDocumentVerifierArtifact] | None = None
    verifier_psm: int | None = Field(default=None, ge=3, le=13)
    verifier_strategy_revision: str | None = None
    verifier_adapter_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    verifier_worker_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    verification_policy_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    orchestrator_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")

    @model_validator(mode="after")
    def validate_verifier_contract(self) -> "HealthDocumentManifest":
        verifier_fields = (
            self.verifier_engine,
            self.verifier_engine_version,
            self.verifier_languages,
            self.verifier_language_artifacts,
            self.verifier_psm,
            self.verifier_strategy_revision,
            self.verifier_adapter_sha256,
            self.verifier_worker_sha256,
            self.verification_policy_sha256,
            self.orchestrator_sha256,
            self.admissibility_policy_sha256,
        )
        if self.verification_revision is None:
            if any(value is not None for value in verifier_fields):
                raise ValueError("verifier fields require verification_revision")
            return self
        if any(value is None for value in verifier_fields):
            raise ValueError("verification_revision requires a complete verifier identity")
        if self.execution_topology_revision != "primary-then-verifier-subprocess-v1":
            raise ValueError(
                "verified extraction requires primary-then-verifier subprocess topology"
            )
        if self.admissibility_policy_revision != "ocr-indicator-admissibility-v2":
            raise ValueError("verified extraction requires admissibility v2")
        declared = set(self.verifier_languages or [])
        artifacts = {item.language for item in self.verifier_language_artifacts or []}
        if declared != artifacts:
            raise ValueError("verifier languages and traineddata artifacts must match exactly")
        return self

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
        return f"hdex-config-{self.fingerprint[:16]}"

    def provenance(self) -> dict[str, object]:
        return {
            "status": "verified",
            "configuration_id": self.configuration_id,
            "mechanism_revision": self.mechanism_revision,
            "output_schema_revision": self.output_schema_revision,
            "execution_topology_revision": self.execution_topology_revision,
            "pdf_strategy_revision": self.pdf_strategy_revision,
            "native_text_engine": self.native_text_engine,
            "native_text_engine_version": self.native_text_engine_version,
            "native_text_quality_policy_revision": self.native_text_quality_policy_revision,
            "native_text_quality_policy_sha256": self.native_text_quality_policy_sha256,
            "ocr_engine": self.ocr_engine,
            "ocr_engine_version": self.ocr_engine_version,
            "runtime_engine": self.runtime_engine,
            "runtime_version": self.runtime_version,
            "model_family": self.model_family,
            "model_type": self.model_type,
            "model_artifacts": [item.model_dump(mode="json") for item in self.model_artifacts],
            "pdf_raster_dpi": self.pdf_raster_dpi,
            "detector_limit_type": self.detector_limit_type,
            "detector_limit_side_len": self.detector_limit_side_len,
            "global_max_side_len": self.global_max_side_len,
            "rec_batch_num": self.rec_batch_num,
            "cls_batch_num": self.cls_batch_num,
            "ort_intra_op_num_threads": self.ort_intra_op_num_threads,
            "ort_inter_op_num_threads": self.ort_inter_op_num_threads,
            "indicator_parser_revision": self.indicator_parser_revision,
            "indicator_parser_sha256": self.indicator_parser_sha256,
            "admissibility_policy_revision": self.admissibility_policy_revision,
            "admissibility_policy_sha256": self.admissibility_policy_sha256,
            "engine_adapter_sha256": self.engine_adapter_sha256,
            "worker_sha256": self.worker_sha256,
            "verification_revision": self.verification_revision,
            "verifier_engine": self.verifier_engine,
            "verifier_engine_version": self.verifier_engine_version,
            "verifier_languages": self.verifier_languages,
            "verifier_language_artifacts": (
                [item.model_dump(mode="json") for item in self.verifier_language_artifacts]
                if self.verifier_language_artifacts is not None
                else None
            ),
            "verifier_psm": self.verifier_psm,
            "verifier_strategy_revision": self.verifier_strategy_revision,
            "verifier_adapter_sha256": self.verifier_adapter_sha256,
            "verifier_worker_sha256": self.verifier_worker_sha256,
            "verification_policy_sha256": self.verification_policy_sha256,
            "orchestrator_sha256": self.orchestrator_sha256,
        }


def _load_yaml(path: Path) -> dict[str, object]:
    payload = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict):
        raise ValueError(f"health-document configuration must be a mapping: {path}")
    return payload


def load_health_document_manifest(path: Path) -> HealthDocumentManifest:
    return HealthDocumentManifest.model_validate(_load_yaml(path))


@lru_cache(maxsize=1)
def get_default_health_document_configuration() -> HealthDocumentManifest:
    return load_health_document_manifest(DEFAULT_MANIFEST_PATH)


@lru_cache(maxsize=16)
def get_health_document_configuration(configuration_id: str) -> HealthDocumentManifest:
    for path in sorted(CONFIG_ROOT.glob("health-document-*.yaml")):
        config = load_health_document_manifest(path)
        if config.configuration_id == configuration_id:
            return config
    raise ValueError(f"unknown health-document configuration_id: {configuration_id}")
