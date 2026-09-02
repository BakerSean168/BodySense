"""OCR-related Pydantic models."""

from typing import Literal

from pydantic import BaseModel, Field

OCRConfidence = Literal["high", "medium", "low", "unknown"]
IndicatorAdmissibilityStatus = Literal["admissible", "needs_review", "rejected"]
IndicatorVerificationStatus = Literal[
    "not_required",
    "pending",
    "verified_consensus",
    "disagreement",
]


class IndicatorEvidenceAdmissibility(BaseModel):
    """Deterministic evidence-admissibility decision for one OCR indicator."""

    status: IndicatorAdmissibilityStatus = "needs_review"
    policy_revision: str = "ocr-indicator-admissibility-v1"
    reason_codes: list[str] = Field(default_factory=lambda: ["not_evaluated"])


class IndicatorEvidenceVerification(BaseModel):
    """Independent verification state for one source-grounded indicator."""

    status: IndicatorVerificationStatus = "pending"
    revision: str = "health-document-row-verification-v1"
    reason_codes: list[str] = Field(default_factory=lambda: ["not_evaluated"])


class HealthIndicator(BaseModel):
    """A health indicator extracted from a report."""

    indicator_id: str | None = Field(default=None, description="Canonical indicator identity")
    name: str = Field(..., min_length=1, description="Name of the indicator (e.g., 'Vitamin D')")
    value: str = Field(..., min_length=1, description="Measured value")
    unit: str | None = Field(None, description="Unit of measurement")
    reference_range: str | None = Field(None, description="Normal reference range")
    confidence: OCRConfidence = Field(
        default="unknown",
        description="Extraction confidence: high, medium, low, or unknown",
    )
    source_refs: list[str] = Field(
        default_factory=list, description="Document source block references"
    )
    source_page: int | None = Field(default=None, ge=1, description="1-based source page")
    parser_revision: str | None = Field(default=None, description="Deterministic parser revision")
    evidence_verification: IndicatorEvidenceVerification | None = Field(
        default=None,
        description="Independent verification state for automatic evidence admission",
    )
    evidence_admissibility: IndicatorEvidenceAdmissibility = Field(
        default_factory=IndicatorEvidenceAdmissibility,
        description="Whether this OCR indicator may be used as health evidence",
    )


class DocumentSourceBlock(BaseModel):
    """Location metadata for one source-grounded text block."""

    source_ref: str = Field(min_length=1)
    page: int = Field(ge=1)
    method: Literal["native_pdf_text", "rapidocr"]
    bbox: list[float] | None = None
    coordinate_space: Literal["pdf_points", "ocr_pixels"]
    confidence: float | None = Field(default=None, ge=0.0, le=1.0)
    text_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class DocumentPageEvidence(BaseModel):
    """Extraction method and quality decision for one page."""

    page: int = Field(ge=1)
    method: Literal["native_pdf_text", "rapidocr"]
    source_refs: list[str] = Field(default_factory=list)
    confidence: float = Field(ge=0.0, le=1.0)
    native_text_quality_policy_revision: str | None = None
    native_text_quality_reason_codes: list[str] = Field(default_factory=list)


class HealthDocumentModelArtifactProvenance(BaseModel):
    role: Literal["det", "rec", "cls"]
    filename: str
    sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class HealthDocumentMechanismProvenance(BaseModel):
    """Immutable non-LLM mechanism identity for health-document extraction."""

    status: Literal["verified"]
    configuration_id: str = Field(pattern=r"^hdex-config-[0-9a-f]{16}$")
    mechanism_revision: str
    output_schema_revision: str
    execution_topology_revision: str
    pdf_strategy_revision: str
    native_text_engine: str
    native_text_engine_version: str
    native_text_quality_policy_revision: str
    native_text_quality_policy_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    ocr_engine: str
    ocr_engine_version: str
    runtime_engine: str
    runtime_version: str
    model_family: str
    model_type: str
    model_artifacts: list[HealthDocumentModelArtifactProvenance]
    pdf_raster_dpi: int
    detector_limit_type: str
    detector_limit_side_len: int
    global_max_side_len: int | None = None
    rec_batch_num: int | None = None
    cls_batch_num: int | None = None
    ort_intra_op_num_threads: int | None = None
    ort_inter_op_num_threads: int | None = None
    indicator_parser_revision: str
    indicator_parser_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    admissibility_policy_revision: str
    admissibility_policy_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    engine_adapter_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    worker_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    verification_revision: str | None = None
    verifier_engine: str | None = None
    verifier_engine_version: str | None = None
    verifier_languages: list[str] | None = None
    verifier_language_artifacts: list[dict[str, str]] | None = None
    verifier_psm: int | None = None
    verifier_strategy_revision: str | None = None
    verifier_adapter_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    verifier_worker_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    verification_policy_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")
    orchestrator_sha256: str | None = Field(default=None, pattern=r"^[0-9a-f]{64}$")


class LegacyTesseractMechanismProvenance(BaseModel):
    """Historical v0.10.2 Tesseract mechanism executed through the current worker."""

    status: Literal["verified"]
    configuration_id: Literal["hdex-config-14af808ef184bf8b"]
    mechanism_revision: Literal["health-document-tesseract-baseline-v1"]
    execution_topology_revision: Literal["per-document-subprocess-v1"]
    engine: Literal["tesseract"]
    engine_version: Literal["5.5.0"]
    wrapper: Literal["pytesseract"]
    wrapper_version: Literal["0.3.13"]
    languages: list[str]
    language_artifacts: list[dict[str, str]]
    pdf_strategy_revision: Literal["raster-all-pages-300dpi-v1"]
    pdf_raster_dpi: Literal[300]
    indicator_parser_revision: Literal["legacy-regex-v1"]
    indicator_parser_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    admissibility_policy_revision: Literal["ocr-indicator-admissibility-v1"]
    ocr_service_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")
    worker_sha256: str = Field(pattern=r"^[0-9a-f]{64}$")


class HealthDocumentVerifierIndicator(BaseModel):
    indicator_id: str = Field(min_length=1)
    page: int = Field(ge=1)
    value: str = Field(min_length=1)
    unit: str | None = None
    reference_range: str | None = None


class HealthDocumentVerifierRow(BaseModel):
    page: int = Field(ge=1)
    y_center: float = Field(ge=0)
    height: float = Field(gt=0)
    value: str = Field(min_length=1)
    unit: str | None = None
    reference_range: str | None = None


class HealthDocumentVerifierResponse(BaseModel):
    status: Literal["verified"] = "verified"
    configuration_id: str = Field(pattern=r"^hdex-config-[0-9a-f]{16}$")
    verification_revision: str = Field(min_length=1)
    indicators: list[HealthDocumentVerifierIndicator] = Field(default_factory=list)
    rows: list[HealthDocumentVerifierRow] = Field(default_factory=list)


class OCRResult(BaseModel):
    """Structured OCR extraction result."""

    raw_text: str = Field(..., description="Full extracted text")
    indicators: list[HealthIndicator] = Field(
        default_factory=list,
        description="Extracted health indicators",
    )
    confidence: OCRConfidence = Field(
        default="unknown",
        description="Overall OCR/extraction confidence",
    )
    mechanism_provenance: (
        HealthDocumentMechanismProvenance | LegacyTesseractMechanismProvenance | None
    ) = None
    pages: list[DocumentPageEvidence] = Field(default_factory=list)
    source_blocks: list[DocumentSourceBlock] = Field(default_factory=list)


class OCRResponse(BaseModel):
    """Response from OCR extraction endpoint."""

    status: str = "completed"
    result: OCRResult


class TextExtractionResponse(BaseModel):
    """Response from text-only extraction endpoint."""

    text: str
    pages: int = Field(default=1, description="Number of pages processed")
