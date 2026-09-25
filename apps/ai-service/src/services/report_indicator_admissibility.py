"""Versioned evidence-admissibility policies for document-derived indicators.

Extraction completion never implies evidence authority. V1 preserves the
historical confidence-only policy. V2 additionally requires independent
verification for OCR-derived rows while allowing native PDF text to declare
verification not required. Neither policy changes extracted values.
"""

from __future__ import annotations

import hashlib
from pathlib import Path

from ..models.ocr import HealthIndicator, IndicatorEvidenceAdmissibility, OCRConfidence

OCR_INDICATOR_ADMISSIBILITY_POLICY_V1 = "ocr-indicator-admissibility-v1"
OCR_INDICATOR_ADMISSIBILITY_POLICY_V2 = "ocr-indicator-admissibility-v2"
# Backward-compatible symbol used by Assessment v4 and historical tests.
OCR_INDICATOR_ADMISSIBILITY_POLICY_REVISION = OCR_INDICATOR_ADMISSIBILITY_POLICY_V1


def admissibility_policy_source_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def _base_reasons(indicator: HealthIndicator, ocr_confidence: OCRConfidence) -> list[str]:
    reasons: list[str] = []
    if ocr_confidence != "high":
        reasons.append(f"ocr_confidence_{ocr_confidence}")
    if indicator.confidence != "high":
        reasons.append(f"indicator_confidence_{indicator.confidence}")
    return reasons


def _malformed(
    indicator: HealthIndicator, policy_revision: str
) -> IndicatorEvidenceAdmissibility | None:
    if indicator.name.strip() and indicator.value.strip():
        return None
    return IndicatorEvidenceAdmissibility(
        status="rejected",
        policy_revision=policy_revision,
        reason_codes=["malformed_indicator"],
    )


def evaluate_indicator_admissibility(
    indicator: HealthIndicator,
    *,
    ocr_confidence: OCRConfidence,
    policy_revision: str = OCR_INDICATOR_ADMISSIBILITY_POLICY_V1,
) -> IndicatorEvidenceAdmissibility:
    """Return a fail-closed evidence decision for one extracted indicator."""

    malformed = _malformed(indicator, policy_revision)
    if malformed is not None:
        return malformed

    reasons = _base_reasons(indicator, ocr_confidence)
    if policy_revision == OCR_INDICATOR_ADMISSIBILITY_POLICY_V1:
        if reasons:
            return IndicatorEvidenceAdmissibility(
                status="needs_review",
                policy_revision=policy_revision,
                reason_codes=reasons,
            )
        return IndicatorEvidenceAdmissibility(
            status="admissible",
            policy_revision=policy_revision,
            reason_codes=["high_confidence_ocr_and_indicator"],
        )

    if policy_revision != OCR_INDICATOR_ADMISSIBILITY_POLICY_V2:
        raise ValueError(f"unsupported OCR indicator admissibility policy: {policy_revision}")

    verification = indicator.evidence_verification
    if verification is None:
        reasons.append("verification_missing")
    elif verification.status == "pending":
        reasons.append("verification_pending")
    elif verification.status == "disagreement":
        reasons.append("independent_verification_disagreement")
    elif verification.status not in {"not_required", "verified_consensus"}:
        reasons.append(f"verification_{verification.status}")

    if reasons:
        return IndicatorEvidenceAdmissibility(
            status="needs_review",
            policy_revision=policy_revision,
            reason_codes=reasons,
        )

    reason = (
        "native_text_high_confidence"
        if verification is not None and verification.status == "not_required"
        else "independent_consensus_high_confidence"
    )
    return IndicatorEvidenceAdmissibility(
        status="admissible",
        policy_revision=policy_revision,
        reason_codes=[reason],
    )


def apply_indicator_admissibility(
    indicators: list[HealthIndicator],
    *,
    ocr_confidence: OCRConfidence,
    policy_revision: str = OCR_INDICATOR_ADMISSIBILITY_POLICY_V1,
) -> list[HealthIndicator]:
    """Attach deterministic admissibility metadata without changing extracted values."""

    return [
        indicator.model_copy(
            update={
                "evidence_admissibility": evaluate_indicator_admissibility(
                    indicator,
                    ocr_confidence=ocr_confidence,
                    policy_revision=policy_revision,
                )
            }
        )
        for indicator in indicators
    ]
