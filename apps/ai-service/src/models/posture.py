"""Posture-analysis Pydantic models.

Mirrors the data contract in
docs/plan/active/posture-photo-analysis-plan.md §3 so the AI service, Go
backend and frontend all agree on the shape stored in
`user_uploads.analysis_result`.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

Confidence = Literal["high", "medium", "low"]
Severity = Literal["mild", "moderate", "marked"]
View = Literal["front", "side", "back"]


class PostureMetric(BaseModel):
    """A quantified geometric metric.

    Phase 1 (pure VLM) never emits metrics — the field stays ``None`` to avoid
    hallucinated angles. Phase 2 (pose estimation) fills it from real geometry.
    """

    name: str = Field(..., description="Metric id, e.g. 'craniovertebral_angle'")
    value: float = Field(..., description="Measured value")
    unit: str = Field(..., description="Unit, e.g. 'deg'")


class PostureFinding(BaseModel):
    """A single posture observation for one view."""

    key: str = Field(..., description="Aligned with knowledge-base problem_slug")
    label: str = Field(..., description="Human-readable label")
    severity: Severity = Field(...)
    confidence: Confidence = Field(...)
    evidence: str = Field("", description="Observable evidence from the photo")
    metric: PostureMetric | None = Field(
        default=None,
        description="Quantified metric (Phase 2 only; None in Phase 1)",
    )


class PostureRedFlag(BaseModel):
    """A safety red flag that warrants seeing a professional."""

    category: str
    message: str


class PostureAnalysis(BaseModel):
    """Structured posture analysis for a single view."""

    schema_version: int = 1
    view: View
    overall_confidence: Confidence = "medium"
    findings: list[PostureFinding] = Field(default_factory=list)
    red_flags: list[PostureRedFlag] = Field(default_factory=list)
    summary_markdown: str = Field("", description="Plain-language summary")
    disclaimer: str = Field(..., description="Mandatory medical disclaimer")
    governance: dict[str, object] | None = None
    safety_fallback: str | None = None
    agent_configuration: dict[str, str] | None = None
    execution_provenance: dict[str, str] | None = None


class PostureAnalysisResponse(BaseModel):
    """Response from the posture analysis endpoint."""

    status: str = "completed"
    result: PostureAnalysis
