"""OCR-related Pydantic models."""

from typing import Literal

from pydantic import BaseModel, Field

OCRConfidence = Literal["high", "medium", "low", "unknown"]
IndicatorAdmissibilityStatus = Literal["admissible", "needs_review", "rejected"]


class IndicatorEvidenceAdmissibility(BaseModel):
    """Deterministic evidence-admissibility decision for one OCR indicator."""

    status: IndicatorAdmissibilityStatus = "needs_review"
    policy_revision: str = "ocr-indicator-admissibility-v1"
    reason_codes: list[str] = Field(default_factory=lambda: ["not_evaluated"])


class HealthIndicator(BaseModel):
    """A health indicator extracted from a report."""

    name: str = Field(..., min_length=1, description="Name of the indicator (e.g., 'Vitamin D')")
    value: str = Field(..., min_length=1, description="Measured value")
    unit: str | None = Field(None, description="Unit of measurement")
    reference_range: str | None = Field(None, description="Normal reference range")
    confidence: OCRConfidence = Field(
        default="unknown",
        description="Extraction confidence: high, medium, low, or unknown",
    )
    evidence_admissibility: IndicatorEvidenceAdmissibility = Field(
        default_factory=IndicatorEvidenceAdmissibility,
        description="Whether this OCR indicator may be used as health evidence",
    )


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


class OCRResponse(BaseModel):
    """Response from OCR extraction endpoint."""

    status: str = "completed"
    result: OCRResult


class TextExtractionResponse(BaseModel):
    """Response from text-only extraction endpoint."""

    text: str
    pages: int = Field(default=1, description="Number of pages processed")
