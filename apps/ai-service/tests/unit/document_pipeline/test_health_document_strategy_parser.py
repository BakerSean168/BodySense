from src.document_pipeline.strategy import (
    NATIVE_TEXT_QUALITY_POLICY_REVISION,
    evaluate_native_text_quality,
    native_text_quality_policy_fingerprint,
)
from src.document_pipeline.structured_indicator_parser import (
    HEALTH_INDICATOR_PARSER_REVISION,
    SourceTextBlock,
    extract_structured_indicators,
)


def test_native_text_quality_accepts_normal_health_report_text() -> None:
    decision = evaluate_native_text_quality(
        "体检报告\n维生素D 17.8 ng/mL 参考范围 30-100\n血红蛋白 142 g/L 参考范围 130-175"
    )
    assert decision.usable is True
    assert decision.policy_revision == NATIVE_TEXT_QUALITY_POLICY_REVISION
    assert decision.reason_codes == ("native_text_usable",)
    assert len(native_text_quality_policy_fingerprint()) == 64


def test_native_text_quality_rejects_empty_short_and_corrupted_text() -> None:
    assert evaluate_native_text_quality("").usable is False
    assert evaluate_native_text_quality("报告 1").usable is False
    corrupted = "\ufffd" * 30 + "report123"
    decision = evaluate_native_text_quality(corrupted)
    assert decision.usable is False
    assert "native_text_contains_replacement_chars" in decision.reason_codes


def test_structured_parser_preserves_numeric_units_ranges_and_source_refs() -> None:
    blocks = [
        SourceTextBlock(
            source_ref="page:1:block:1",
            page=1,
            text=(
                "白细胞: 6.21 10^9/L 参考范围: 3.5-9.5\n"
                "红细胞: 4.82 10^12/L 参考范围: 4.3-5.8\n"
                "维生素D: 17.8 ng/mL 参考范围: 30-100"
            ),
        )
    ]
    indicators = extract_structured_indicators(blocks)
    by_id = {item.indicator_id: item for item in indicators}
    assert by_id["wbc"].value == "6.21"
    assert by_id["wbc"].unit == "10^9/L"
    assert by_id["rbc"].unit == "10^12/L"
    assert by_id["vitamin_d"].reference_range == "30-100"
    assert by_id["vitamin_d"].source_refs == ["page:1:block:1"]
    assert by_id["vitamin_d"].source_page == 1
    assert by_id["vitamin_d"].parser_revision == HEALTH_INDICATOR_PARSER_REVISION
    assert all(item.confidence == "high" for item in indicators)


def test_structured_parser_deduplicates_same_canonical_indicator() -> None:
    blocks = [
        SourceTextBlock("page:1:block:1", 1, "维生素D 17.8 ng/mL 参考范围 30-100"),
        SourceTextBlock("page:2:block:1", 2, "Vitamin D 18.0 ng/mL reference 30-100"),
    ]
    indicators = extract_structured_indicators(blocks)
    assert len([item for item in indicators if item.indicator_id == "vitamin_d"]) == 1


def test_structured_parser_normalizes_pdf_compatibility_glyphs_and_nonbreaking_hyphens() -> None:
    blocks = [
        SourceTextBlock(
            "page:3:native-block:1",
            3,
            "尿酸: 398 umol/L 参考范围: 208‑428\n尿素氮: 5.4 mmol/L 参考范围: 3.1‐8.0",
        )
    ]
    indicators = extract_structured_indicators(blocks)
    by_id = {item.indicator_id: item for item in indicators}
    assert by_id["uric_acid"].value == "398"
    assert by_id["uric_acid"].reference_range == "208-428"
    assert by_id["bun"].value == "5.4"
    assert by_id["bun"].reference_range == "3.1-8.0"


def test_structured_parser_accepts_table_row_without_repeated_reference_label() -> None:
    block = SourceTextBlock(
        source_ref="page:1:ocr-block:1",
        source_refs=(
            "page:1:ocr-block:1",
            "page:1:ocr-block:2",
            "page:1:ocr-block:3",
            "page:1:ocr-block:4",
        ),
        page=1,
        text="维生素D 17.8 ng/mL 30-100",
    )
    indicators = extract_structured_indicators([block])
    assert len(indicators) == 1
    indicator = indicators[0]
    assert indicator.indicator_id == "vitamin_d"
    assert indicator.value == "17.8"
    assert indicator.unit == "ng/mL"
    assert indicator.reference_range == "30-100"
    assert indicator.source_refs == list(block.source_refs)
    assert indicator.confidence == "high"
