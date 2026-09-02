"""Typed contracts for health-document extraction benchmarking.

These contracts deliberately live outside the current OCR service. The benchmark
must be able to compare the production baseline with future candidates without
changing the serving path before a Champion is selected.
"""

from __future__ import annotations

import hashlib
import json
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

CorpusCohort = Literal[
    "native_pdf_simple",
    "native_pdf_table",
    "scanned_pdf_clear",
    "scanned_pdf_degraded",
    "phone_photo_clear",
    "phone_photo_degraded",
    "chinese_lab_table",
    "mixed_zh_en_units",
    "complex_table",
]

SourceClassification = Literal["synthetic", "deidentified"]


class LanguageArtifact(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    language: str = Field(min_length=1)
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class PdfRasterContract(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    engine: str = Field(min_length=1)
    engine_version: str = Field(min_length=1)
    strategy: Literal["raster_all_pages"]
    dpi: int = Field(ge=72, le=600)
    image_format: Literal["png"]


class ModelArtifact(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    role: Literal["det", "rec", "cls"]
    filename: str = Field(min_length=1)
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class SourceContract(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    ocr_service_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    engine_adapter_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    indicator_extractor_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    admissibility_policy_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class BenchmarkCandidateConfig(BaseModel):
    """Exact behavior identity for one benchmark candidate.

    The configuration is intentionally stricter than a human-readable candidate
    label. The fingerprint includes runtime/model artifacts and the current
    repository-owned extraction/parser source hashes, so a silent behavior change
    cannot keep the same benchmark identity.
    """

    model_config = ConfigDict(frozen=True, extra="forbid")

    schema_version: Literal["health-document-candidate-v1"]
    candidate_id: str = Field(min_length=1)
    label: str = Field(min_length=1)
    mechanism_revision: str = Field(min_length=1)
    engine: Literal["tesseract", "rapidocr"]
    engine_version: str = Field(min_length=1)
    wrapper: Literal["pytesseract", "rapidocr"]
    wrapper_version: str = Field(min_length=1)
    pillow_version: str = Field(min_length=1)
    languages: list[str] = Field(min_length=1)
    language_artifacts: list[LanguageArtifact] = Field(default_factory=list)
    tesseract_config: str | None = None
    runtime_engine: Literal["onnxruntime"] | None = None
    runtime_version: str | None = None
    model_family: str | None = None
    model_type: str | None = None
    model_artifacts: list[ModelArtifact] | None = None
    engine_parameters: dict[str, str] | None = None
    pdf_raster: PdfRasterContract
    source_contract: SourceContract

    @model_validator(mode="after")
    def validate_engine_contract(self) -> "BenchmarkCandidateConfig":
        if self.engine == "tesseract":
            declared = set(self.languages)
            artifact_languages = {artifact.language for artifact in self.language_artifacts}
            if declared != artifact_languages:
                raise ValueError("Tesseract languages and language_artifacts must match exactly")
            if not self.source_contract.ocr_service_sha256:
                raise ValueError("Tesseract candidate requires ocr_service_sha256")
            if self.tesseract_config is None:
                raise ValueError("Tesseract candidate requires tesseract_config")
        elif self.engine == "rapidocr":
            required = (
                self.runtime_engine,
                self.runtime_version,
                self.model_family,
                self.model_type,
                self.model_artifacts,
                self.engine_parameters,
                self.source_contract.engine_adapter_sha256,
            )
            if any(value is None for value in required):
                raise ValueError(
                    "RapidOCR candidate requires runtime/model/artifact/adapter identity"
                )
            if not self.model_artifacts:
                raise ValueError("RapidOCR candidate requires model_artifacts")
            if {artifact.role for artifact in self.model_artifacts} != {"det", "rec", "cls"}:
                raise ValueError("RapidOCR candidate requires det/rec/cls model artifacts")
        return self

    def canonical_behavior_json(self) -> str:
        return json.dumps(
            self.model_dump(mode="json", exclude={"label"}, exclude_none=True),
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


class CorpusIndicatorTruth(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    indicator_id: str = Field(min_length=1)
    display_name: str = Field(min_length=1)
    value: str = Field(min_length=1)
    unit: str | None = None
    reference_range: str | None = None
    page: int = Field(ge=1)
    row: int = Field(ge=1)
    critical_numeric: bool = True


class CorpusReviewAttestation(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    reviewer_id: str = Field(pattern=r"^reviewer-[a-z0-9][a-z0-9-]*$")
    truth_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    deidentification_attested: Literal[True]


class CorpusDocument(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")

    schema_version: Literal["health-document-corpus-item-v1"]
    fixture_id: str = Field(pattern=r"^[a-z0-9][a-z0-9-]+$")
    cohort: CorpusCohort
    asset_path: str = Field(min_length=1)
    mime_type: Literal["application/pdf", "image/png", "image/jpeg"]
    page_count: int = Field(ge=1)
    source_classification: SourceClassification
    contains_real_user_data: Literal[False]
    source_license: str = Field(min_length=1)
    generator_revision: str | None = None
    generator_asset_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    asset_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    annotation_state: Literal[
        "synthetic_ground_truth",
        "unreviewed",
        "single_reviewed",
        "double_reviewed",
    ]
    indicators: list[CorpusIndicatorTruth]
    review_attestations: list[CorpusReviewAttestation] = Field(default_factory=list)

    def truth_sha256(self) -> str:
        canonical = json.dumps(
            [
                item.model_dump(mode="json")
                for item in sorted(
                    self.indicators,
                    key=lambda value: (value.page, value.row, value.indicator_id),
                )
            ],
            ensure_ascii=False,
            sort_keys=True,
            separators=(",", ":"),
        )
        return hashlib.sha256(canonical.encode()).hexdigest()

    @model_validator(mode="after")
    def validate_source_policy(self) -> "CorpusDocument":
        if self.source_classification == "synthetic":
            if self.annotation_state != "synthetic_ground_truth":
                raise ValueError("synthetic fixtures must use synthetic_ground_truth annotations")
            if self.review_attestations:
                raise ValueError("synthetic fixtures cannot carry human review attestations")
            if self.asset_sha256 is not None:
                raise ValueError("synthetic fixtures use generator_asset_sha256, not asset_sha256")
        else:
            if self.annotation_state == "synthetic_ground_truth":
                raise ValueError("deidentified fixtures require human review state")
            if self.asset_sha256 is None:
                raise ValueError("deidentified fixtures require asset_sha256")
            reviewer_ids = [item.reviewer_id for item in self.review_attestations]
            if len(set(reviewer_ids)) != len(reviewer_ids):
                raise ValueError("review attestations require distinct reviewer_id values")
            truth_sha = self.truth_sha256()
            if any(item.truth_sha256 != truth_sha for item in self.review_attestations):
                raise ValueError(
                    "review attestation truth_sha256 does not match current annotation truth"
                )
            if self.annotation_state == "unreviewed" and self.review_attestations:
                raise ValueError("unreviewed fixtures cannot carry review attestations")
            if self.annotation_state == "single_reviewed" and len(self.review_attestations) != 1:
                raise ValueError("single_reviewed fixtures require exactly one review attestation")
            if self.annotation_state == "double_reviewed" and len(self.review_attestations) < 2:
                raise ValueError(
                    "double_reviewed fixtures require two independent review attestations"
                )
        if any(indicator.page > self.page_count for indicator in self.indicators):
            raise ValueError("indicator page exceeds document page_count")
        return self


class PredictedIndicator(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    name: str
    value: str
    unit: str | None = None
    reference_range: str | None = None
    confidence: str | None = None
    admissibility_status: Literal["admissible", "needs_review", "rejected"] | None = None
    verification_status: (
        Literal["not_required", "pending", "verified_consensus", "disagreement"] | None
    ) = None


class SourceAccuracyCounts(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    truth_indicators: int = Field(ge=0)
    name_present: int = Field(ge=0)
    numeric_exact: int = Field(ge=0)
    unit_exact: int = Field(ge=0)
    reference_range_exact: int = Field(ge=0)
    row_bundle_exact: int = Field(ge=0)
    critical_indicators: int = Field(ge=0)
    critical_numeric_errors: int = Field(ge=0)


class SourceAccuracySummary(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    truth_indicators: int = Field(ge=0)
    name_recall: float = Field(ge=0, le=1)
    numeric_exact_match: float = Field(ge=0, le=1)
    unit_exact_match: float = Field(ge=0, le=1)
    reference_range_exact_match: float = Field(ge=0, le=1)
    row_bundle_exact_match: float = Field(ge=0, le=1)
    critical_numeric_error_rate: float = Field(ge=0, le=1)


class FixtureBenchmarkResult(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    fixture_id: str
    cohort: CorpusCohort
    elapsed_ms: float = Field(ge=0)
    raw_text_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    predicted_indicators: list[PredictedIndicator]
    source_counts: SourceAccuracyCounts | None = None
    error: str | None = None


class AccuracySummary(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    truth_indicators: int = Field(ge=0)
    matched_indicators: int = Field(ge=0)
    numeric_exact_match: float = Field(ge=0, le=1)
    unit_exact_match: float = Field(ge=0, le=1)
    reference_range_exact_match: float = Field(ge=0, le=1)
    indicator_exact_match: float = Field(ge=0, le=1)
    row_association_accuracy: float = Field(ge=0, le=1)
    critical_numeric_error_rate: float = Field(ge=0, le=1)


class EvidenceAuthoritySummary(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    truth_indicators: int = Field(ge=0)
    auto_admitted: int = Field(ge=0)
    exact_auto_admitted: int = Field(ge=0)
    needs_review: int = Field(ge=0)
    verification_disagreements: int = Field(ge=0)
    critical_false_admissions: int = Field(ge=0)
    auto_admission_coverage: float = Field(ge=0, le=1)
    auto_admission_exact_rate: float = Field(ge=0, le=1)
    critical_false_admission_rate: float = Field(ge=0, le=1)


class RuntimeSummary(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    total_elapsed_ms: float = Field(ge=0)
    mean_fixture_ms: float = Field(ge=0)
    p95_fixture_ms: float = Field(ge=0)
    peak_self_rss_mb: float = Field(ge=0)
    peak_child_rss_mb: float = Field(ge=0)
    cgroup_memory_limit_mb: float | None = Field(default=None, gt=0)
    cgroup_memory_peak_mb: float | None = Field(default=None, ge=0)
    cgroup_memory_events_max: int | None = Field(default=None, ge=0)
    cgroup_memory_events_oom: int | None = Field(default=None, ge=0)
    cgroup_memory_events_oom_kill: int | None = Field(default=None, ge=0)
    cgroup_swap_peak_mb: float | None = Field(default=None, ge=0)
    cgroup_cpu_limit: float | None = Field(default=None, gt=0)


class BenchmarkRunResult(BaseModel):
    model_config = ConfigDict(frozen=True, extra="forbid")
    schema_version: Literal["health-document-benchmark-run-v1"]
    harness_revision: Literal[
        "health-document-benchmark-v1",
        "health-document-benchmark-v2",
        "health-document-benchmark-v3",
        "health-document-benchmark-v4",
    ]
    candidate_id: str
    configuration_id: str
    candidate_fingerprint: str = Field(pattern=r"^[0-9a-f]{64}$")
    corpus_manifest_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    production_shaped: bool
    execution_topology_revision: Literal[
        "in-process-v1",
        "per-document-subprocess-v1",
        "primary-then-verifier-subprocess-v1",
    ] = "in-process-v1"
    fixture_results: list[FixtureBenchmarkResult]
    source_accuracy: SourceAccuracySummary | None = None
    accuracy: AccuracySummary
    runtime: RuntimeSummary
