"""Independent consensus and evidence-admissibility preparation for health documents.

This module never corrects a primary OCR value. It only compares independently
extracted structured indicators and records whether automatic evidence admission
is safe. Disagreement remains reviewable evidence, not an auto-corrected fact.
"""

from __future__ import annotations

import hashlib
import re
from pathlib import Path

from ..models.ocr import (
    DocumentSourceBlock,
    HealthDocumentVerifierResponse,
    HealthIndicator,
    IndicatorEvidenceVerification,
)

ROW_VERIFICATION_REVISION = "health-document-row-verification-v2-geometry"


def verification_policy_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def _norm(value: str | None) -> str:
    if value is None:
        return ""
    return (
        re.sub(r"\s+", "", value)
        .replace("µ", "u")
        .replace("μ", "u")
        .replace("～", "-")
        .replace("~", "-")
        .replace("—", "-")
        .replace("–", "-")
        .lower()
    )


def _signature(indicator: HealthIndicator) -> tuple[str, str, str]:
    return (
        _norm(indicator.value),
        _norm(indicator.unit),
        _norm(indicator.reference_range),
    )


def mark_initial_verification(
    indicators: list[HealthIndicator],
    *,
    page_methods: dict[int, str],
    verification_revision: str = ROW_VERIFICATION_REVISION,
) -> list[HealthIndicator]:
    """Mark native evidence as self-authenticating and OCR evidence as pending."""

    updated: list[HealthIndicator] = []
    for indicator in indicators:
        method = page_methods.get(indicator.source_page or 0)
        if method == "native_pdf_text":
            verification = IndicatorEvidenceVerification(
                status="not_required",
                revision=verification_revision,
                reason_codes=["native_pdf_text"],
            )
        else:
            verification = IndicatorEvidenceVerification(
                status="pending",
                revision=verification_revision,
                reason_codes=["independent_ocr_verification_pending"],
            )
        updated.append(indicator.model_copy(update={"evidence_verification": verification}))
    return updated


def _primary_row_geometry(
    indicator: HealthIndicator,
    source_blocks: list[DocumentSourceBlock],
) -> tuple[float, float] | None:
    refs = set(indicator.source_refs)
    boxes = [block.bbox for block in source_blocks if block.source_ref in refs and block.bbox]
    if not boxes:
        return None
    ys = [float(value) for box in boxes for value in box[1::2]]
    if not ys:
        return None
    y_min, y_max = min(ys), max(ys)
    return (y_min + y_max) / 2.0, max(1.0, y_max - y_min)


def _geometry_verifier_signature(
    indicator: HealthIndicator,
    verifier: HealthDocumentVerifierResponse,
    source_blocks: list[DocumentSourceBlock],
) -> tuple[str, str, str] | None:
    geometry = _primary_row_geometry(indicator, source_blocks)
    if geometry is None or indicator.source_page is None:
        return None
    primary_y, primary_height = geometry
    candidates = [row for row in verifier.rows if row.page == indicator.source_page]
    if not candidates:
        return None
    nearest = min(candidates, key=lambda row: abs(row.y_center - primary_y))
    tolerance = max(8.0, max(primary_height, nearest.height) * 0.8)
    if abs(nearest.y_center - primary_y) > tolerance:
        return None
    return (
        _norm(nearest.value),
        _norm(nearest.unit),
        _norm(nearest.reference_range),
    )


def apply_verifier_consensus(
    indicators: list[HealthIndicator],
    verifier: HealthDocumentVerifierResponse,
    *,
    page_methods: dict[int, str],
    source_blocks: list[DocumentSourceBlock] | None = None,
) -> list[HealthIndicator]:
    """Attach exact structured consensus without replacing the primary value."""

    by_key = {(item.indicator_id, item.page): item for item in verifier.indicators}
    source_blocks = source_blocks or []
    verification_revision = verifier.verification_revision
    updated: list[HealthIndicator] = []
    for indicator in indicators:
        page = indicator.source_page or 0
        if page_methods.get(page) == "native_pdf_text":
            verification = IndicatorEvidenceVerification(
                status="not_required",
                revision=verification_revision,
                reason_codes=["native_pdf_text"],
            )
        elif not indicator.indicator_id:
            verification = IndicatorEvidenceVerification(
                status="disagreement",
                revision=verification_revision,
                reason_codes=["canonical_indicator_id_missing"],
            )
        else:
            secondary = by_key.get((indicator.indicator_id, page))
            secondary_signature: tuple[str, str, str] | None = None
            reason = "verifier_indicator_missing"
            if secondary is not None:
                secondary_signature = (
                    _norm(secondary.value),
                    _norm(secondary.unit),
                    _norm(secondary.reference_range),
                )
                reason = "independent_structured_disagreement"
                primary_signature = _signature(indicator)
                secondary_value = secondary_signature[0]
                # A conflicting independently read numeric value is a hard
                # disagreement. Geometry must never override it. If the value
                # agrees but the structured verifier could not parse unit/range,
                # a geometry-aligned row from the same independent OCR pass may
                # complete the signature.
                if secondary_value == primary_signature[0] and (
                    not secondary_signature[1] or not secondary_signature[2]
                ):
                    geometry_signature = _geometry_verifier_signature(
                        indicator,
                        verifier,
                        source_blocks,
                    )
                    if geometry_signature is not None:
                        secondary_signature = geometry_signature
                        reason = "independent_geometry_disagreement"
            else:
                secondary_signature = _geometry_verifier_signature(
                    indicator,
                    verifier,
                    source_blocks,
                )
                if secondary_signature is not None:
                    reason = "independent_geometry_disagreement"

            if secondary_signature is not None and _signature(indicator) == secondary_signature:
                verification = IndicatorEvidenceVerification(
                    status="verified_consensus",
                    revision=verification_revision,
                    reason_codes=[
                        "independent_structured_consensus"
                        if secondary is not None
                        else "independent_geometry_consensus"
                    ],
                )
            else:
                verification = IndicatorEvidenceVerification(
                    status="disagreement",
                    revision=verification_revision,
                    reason_codes=[reason],
                )
        updated.append(indicator.model_copy(update={"evidence_verification": verification}))
    return updated


def finalize_verified_response(
    response,
    verifier: HealthDocumentVerifierResponse,
    *,
    policy_revision: str,
):
    """Finalize verification and admissibility after the independent worker exits."""

    from ..services.report_indicator_admissibility import apply_indicator_admissibility

    page_methods = {page.page: page.method for page in response.result.pages}
    verified = apply_verifier_consensus(
        response.result.indicators,
        verifier,
        page_methods=page_methods,
        source_blocks=response.result.source_blocks,
    )
    admitted = apply_indicator_admissibility(
        verified,
        ocr_confidence=response.result.confidence,
        policy_revision=policy_revision,
    )
    return response.model_copy(
        update={"result": response.result.model_copy(update={"indicators": admitted})}
    )
