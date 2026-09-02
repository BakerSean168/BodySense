"""Deterministic admissibility policy for OCR-derived health indicators.

OCR completion means extraction finished; it does not mean every parsed value is
safe to use as health evidence. This module converts extraction confidence into
an explicit, versioned evidence decision. Only auto-admissible indicators may
enter the current Assessment evidence catalog. Review-required indicators stay
persisted/displayable but cannot influence durable health observations.
"""

from __future__ import annotations

from ..models.ocr import HealthIndicator, IndicatorEvidenceAdmissibility, OCRConfidence

OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION = "ocr-indicator-admissibility-v1"


def evaluate_indicator_admissibility(
    indicator: HealthIndicator,
    *,
    ocr_confidence: OCRConfidence,
) -> IndicatorEvidenceAdmissibility:
    """Return a fail-closed evidence decision for one extracted indicator."""

    reasons: list[str] = []
    if not indicator.name.strip() or not indicator.value.strip():
        return IndicatorEvidenceAdmissibility(
            status="rejected",
            policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
            reason_codes=["malformed_indicator"],
        )

    if ocr_confidence != "high":
        reasons.append(f"ocr_confidence_{ocr_confidence}")
    if indicator.confidence != "high":
        reasons.append(f"indicator_confidence_{indicator.confidence}")

    if reasons:
        return IndicatorEvidenceAdmissibility(
            status="needs_review",
            policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
            reason_codes=reasons,
        )

    return IndicatorEvidenceAdmissibility(
        status="admissible",
        policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION,
        reason_codes=["high_confidence_ocr_and_indicator"],
    )


def apply_indicator_admissibility(
    indicators: list[HealthIndicator],
    *,
    ocr_confidence: OCRConfidence,
) -> list[HealthIndicator]:
    """Attach deterministic admissibility metadata without changing extracted values."""

    return [
        indicator.model_copy(
            update={
                "evidence_admissibility": evaluate_indicator_admissibility(
                    indicator,
                    ocr_confidence=ocr_confidence,
                )
            }
        )
        for indicator in indicators
    ]
