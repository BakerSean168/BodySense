from src.document_pipeline.verification import (
    ROW_VERIFICATION_REVISION,
    apply_verifier_consensus,
    mark_initial_verification,
)
from src.models.ocr import (
    HealthDocumentVerifierIndicator,
    HealthDocumentVerifierResponse,
    HealthIndicator,
    IndicatorEvidenceVerification,
)
from src.services.report_indicator_admissibility import (
    OCR_INDICATOR_ADMISSIBILITY_POLICY_V1,
    OCR_INDICATOR_ADMISSIBILITY_POLICY_V2,
    evaluate_indicator_admissibility,
)


def _indicator(value: str = "398") -> HealthIndicator:
    return HealthIndicator(
        indicator_id="uric_acid",
        name="尿酸",
        value=value,
        unit="umol/L",
        reference_range="208-428",
        confidence="high",
        source_refs=["page:1:ocr-block:1"],
        source_page=1,
        parser_revision="health-indicator-parser-v3-table-rows",
    )


def test_v1_preserves_historical_confidence_only_admission() -> None:
    result = evaluate_indicator_admissibility(
        _indicator("898"),
        ocr_confidence="high",
        policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_V1,
    )
    assert result.status == "admissible"


def test_v2_requires_independent_consensus_for_ocr_rows() -> None:
    indicator = _indicator("398").model_copy(
        update={
            "evidence_verification": IndicatorEvidenceVerification(
                status="verified_consensus",
                revision=ROW_VERIFICATION_REVISION,
                reason_codes=["independent_structured_consensus"],
            )
        }
    )
    result = evaluate_indicator_admissibility(
        indicator,
        ocr_confidence="high",
        policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_V2,
    )
    assert result.status == "admissible"
    assert result.reason_codes == ["independent_consensus_high_confidence"]


def test_v2_disagreement_never_auto_corrects_primary_value() -> None:
    primary = mark_initial_verification([_indicator("898")], page_methods={1: "rapidocr"})
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision=ROW_VERIFICATION_REVISION,
        indicators=[
            HealthDocumentVerifierIndicator(
                indicator_id="uric_acid",
                page=1,
                value="398",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    compared = apply_verifier_consensus(primary, verifier, page_methods={1: "rapidocr"})
    assert compared[0].value == "898"
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.status == "disagreement"
    decision = evaluate_indicator_admissibility(
        compared[0],
        ocr_confidence="high",
        policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_V2,
    )
    assert decision.status == "needs_review"
    assert "independent_verification_disagreement" in decision.reason_codes


def test_v2_native_pdf_text_does_not_require_secondary_ocr() -> None:
    marked = mark_initial_verification([_indicator()], page_methods={1: "native_pdf_text"})
    assert marked[0].evidence_verification is not None
    assert marked[0].evidence_verification.status == "not_required"
    decision = evaluate_indicator_admissibility(
        marked[0],
        ocr_confidence="high",
        policy_revision=OCR_INDICATOR_ADMISSIBILITY_POLICY_V2,
    )
    assert decision.status == "admissible"
    assert decision.reason_codes == ["native_text_high_confidence"]


def test_geometry_consensus_can_verify_when_secondary_name_is_unusable() -> None:
    from src.models.ocr import DocumentSourceBlock, HealthDocumentVerifierRow

    primary = mark_initial_verification([_indicator("398")], page_methods={1: "rapidocr"})
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision=ROW_VERIFICATION_REVISION,
        rows=[
            HealthDocumentVerifierRow(
                page=1,
                y_center=120.0,
                height=30.0,
                value="398",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    source_blocks = [
        DocumentSourceBlock(
            source_ref="page:1:ocr-block:1",
            page=1,
            method="rapidocr",
            bbox=[10, 105, 100, 105, 100, 135, 10, 135],
            coordinate_space="ocr_pixels",
            confidence=0.99,
            text_sha256="1" * 64,
        )
    ]
    compared = apply_verifier_consensus(
        primary,
        verifier,
        page_methods={1: "rapidocr"},
        source_blocks=source_blocks,
    )
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.status == "verified_consensus"
    assert compared[0].evidence_verification.reason_codes == ["independent_geometry_consensus"]


def test_geometry_alignment_never_overrides_a_numeric_disagreement() -> None:
    from src.models.ocr import DocumentSourceBlock, HealthDocumentVerifierRow

    primary = mark_initial_verification([_indicator("898")], page_methods={1: "rapidocr"})
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision=ROW_VERIFICATION_REVISION,
        rows=[
            HealthDocumentVerifierRow(
                page=1,
                y_center=120.0,
                height=30.0,
                value="398",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    source_blocks = [
        DocumentSourceBlock(
            source_ref="page:1:ocr-block:1",
            page=1,
            method="rapidocr",
            bbox=[10, 105, 100, 105, 100, 135, 10, 135],
            coordinate_space="ocr_pixels",
            confidence=0.99,
            text_sha256="1" * 64,
        )
    ]
    compared = apply_verifier_consensus(
        primary,
        verifier,
        page_methods={1: "rapidocr"},
        source_blocks=source_blocks,
    )
    assert compared[0].value == "898"
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.status == "disagreement"


def test_verifier_measurement_parser_accepts_explicit_reference_range_label() -> None:
    from src.document_pipeline.row_verifier import _measurement_signature

    assert _measurement_signature("血糖 : 5.18 mmol/L 参考 范围 : 3.9-6.1") == (
        "5.18",
        "mmol/L",
        "3.9-6.1",
    )
    assert _measurement_signature("尿酸 : 398 umol/L 参考范围: 208-428") == (
        "398",
        "umol/L",
        "208-428",
    )


def test_verification_revision_follows_actual_verifier_identity() -> None:
    revision = "health-document-row-verification-v3-labeled-range"
    primary = mark_initial_verification(
        [_indicator("398")],
        page_methods={1: "rapidocr"},
        verification_revision=revision,
    )
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision=revision,
        indicators=[
            HealthDocumentVerifierIndicator(
                indicator_id="uric_acid",
                page=1,
                value="398",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    compared = apply_verifier_consensus(primary, verifier, page_methods={1: "rapidocr"})
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.revision == revision
    assert compared[0].evidence_verification.status == "verified_consensus"


def test_verifier_normalizes_degree_sign_only_in_scientific_count_unit() -> None:
    from src.document_pipeline.row_verifier import (
        _canonicalize_tsv_line_text,
        _measurement_signature,
    )

    strategy = "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization"
    rbc = _canonicalize_tsv_line_text("4.82 10°12/L 参考 范围 : 4.3-5.8", strategy)
    platelet = _canonicalize_tsv_line_text("238 10°9/L 参考范围: 125-350", strategy)
    assert _measurement_signature(rbc) == ("4.82", "10^12/L", "4.3-5.8")
    assert _measurement_signature(platelet) == ("238", "10^9/L", "125-350")


def test_incomplete_structured_consensus_may_use_matching_geometry_row() -> None:
    from src.models.ocr import DocumentSourceBlock, HealthDocumentVerifierRow

    primary = mark_initial_verification(
        [_indicator("398")],
        page_methods={1: "rapidocr"},
        verification_revision="health-document-row-verification-v5-scientific-unit-normalization",
    )
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision="health-document-row-verification-v5-scientific-unit-normalization",
        indicators=[
            HealthDocumentVerifierIndicator(
                indicator_id="uric_acid",
                page=1,
                value="398",
                unit=None,
                reference_range=None,
            )
        ],
        rows=[
            HealthDocumentVerifierRow(
                page=1,
                y_center=120.0,
                height=30.0,
                value="398",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    source_blocks = [
        DocumentSourceBlock(
            source_ref="page:1:ocr-block:1",
            page=1,
            method="rapidocr",
            bbox=[10, 105, 100, 105, 100, 135, 10, 135],
            coordinate_space="ocr_pixels",
            confidence=0.99,
            text_sha256="1" * 64,
        )
    ]
    compared = apply_verifier_consensus(
        primary,
        verifier,
        page_methods={1: "rapidocr"},
        source_blocks=source_blocks,
    )
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.status == "verified_consensus"
    assert compared[0].evidence_verification.revision == verifier.verification_revision
    assert compared[0].evidence_verification.reason_codes == ["independent_structured_consensus"]


def test_conflicting_structured_numeric_value_cannot_be_overridden_by_geometry() -> None:
    from src.models.ocr import DocumentSourceBlock, HealthDocumentVerifierRow

    primary = mark_initial_verification(
        [_indicator("898")],
        page_methods={1: "rapidocr"},
        verification_revision="health-document-row-verification-v5-scientific-unit-normalization",
    )
    verifier = HealthDocumentVerifierResponse(
        configuration_id="hdex-config-1111111111111111",
        verification_revision="health-document-row-verification-v5-scientific-unit-normalization",
        indicators=[
            HealthDocumentVerifierIndicator(
                indicator_id="uric_acid",
                page=1,
                value="398",
                unit=None,
                reference_range=None,
            )
        ],
        # Even a geometry row that happens to mirror the primary value must not
        # erase the independent numeric conflict above.
        rows=[
            HealthDocumentVerifierRow(
                page=1,
                y_center=120.0,
                height=30.0,
                value="898",
                unit="umol/L",
                reference_range="208-428",
            )
        ],
    )
    source_blocks = [
        DocumentSourceBlock(
            source_ref="page:1:ocr-block:1",
            page=1,
            method="rapidocr",
            bbox=[10, 105, 100, 105, 100, 135, 10, 135],
            coordinate_space="ocr_pixels",
            confidence=0.99,
            text_sha256="1" * 64,
        )
    ]
    compared = apply_verifier_consensus(
        primary,
        verifier,
        page_methods={1: "rapidocr"},
        source_blocks=source_blocks,
    )
    assert compared[0].value == "898"
    assert compared[0].evidence_verification is not None
    assert compared[0].evidence_verification.status == "disagreement"
