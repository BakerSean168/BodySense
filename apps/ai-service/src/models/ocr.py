"""OCR-related Pydantic models."""

from pydantic import BaseModel, Field


class HealthIndicator(BaseModel):
    """A health indicator extracted from a report."""

    name: str = Field(..., description="Name of the indicator (e.g., 'Vitamin D')")
    value: str = Field(..., description="Measured value")
    unit: str | None = Field(None, description="Unit of measurement")
    reference_range: str | None = Field(None, description="Normal reference range")
    confidence: str = Field(
        default="high",
        description="Confidence level: high, medium, or low",
    )


class OCRResult(BaseModel):
    """Structured OCR extraction result."""

    raw_text: str = Field(..., description="Full extracted text")
    indicators: list[HealthIndicator] = Field(
        default_factory=list,
        description="Extracted health indicators",
    )
    confidence: str = Field(
        default="high",
        description="Overall confidence level",
    )


class OCRResponse(BaseModel):
    """Response from OCR extraction endpoint."""

    status: str = "completed"
    result: OCRResult


class TextExtractionResponse(BaseModel):
    """Response from text-only extraction endpoint."""

    text: str
    pages: int = Field(default=1, description="Number of pages processed")
