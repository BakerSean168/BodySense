from src.document_pipeline.row_verifier import (
    _canonicalize_tsv_line_text,
    _measurement_signature,
)


def test_v2_cjk_row_normalization_joins_only_adjacent_cjk_tokens() -> None:
    raw = "甘油 三 酯 : 1.12 mmol/L 参考 范围 : 0-1.7"
    normalized = _canonicalize_tsv_line_text(
        raw, "full-ocr-page-tsv-geometry-v2-cjk-row-normalization"
    )
    assert normalized == "甘油三酯 : 1.12 mmol/L 参考范围 : 0-1.7"
    assert "1.12" in normalized
    assert "0-1.7" in normalized


def test_v1_strategy_preserves_historical_tsv_spacing() -> None:
    raw = "参考 范围 : 3.9-6.1"
    assert _canonicalize_tsv_line_text(raw, "full-ocr-page-tsv-geometry-v1") == raw


def test_v3_measurement_row_parses_reference_label_without_indicator_name() -> None:
    raw = "未识别名称 78 umol/L 参考 范围 : 57-111"
    normalized = _canonicalize_tsv_line_text(raw, "full-ocr-page-tsv-geometry-v3-measurement-rows")
    assert _measurement_signature(normalized) == ("78", "umol/L", "57-111")


def test_measurement_row_parser_never_changes_numeric_tokens() -> None:
    raw = "尿酸 898 umol/L 参考 范围 : 208-428"
    normalized = _canonicalize_tsv_line_text(raw, "full-ocr-page-tsv-geometry-v3-measurement-rows")
    assert _measurement_signature(normalized) == ("898", "umol/L", "208-428")


def test_v4_scientific_unit_normalization_handles_degree_sign_only() -> None:
    strategy = "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization"

    rbc = _canonicalize_tsv_line_text("红细胞 4.82 10°12/L 4.3-5.8", strategy)
    assert _measurement_signature(rbc) == ("4.82", "10^12/L", "4.3-5.8")

    caret_loss = _canonicalize_tsv_line_text("红细胞 4.82 10412/L 4.3-5.8", strategy)
    assert caret_loss == "红细胞 4.82 10412/L 4.3-5.8"


def test_v5_scientific_unit_ocr_normalization_handles_caret_loss_token() -> None:
    strategy = "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization"

    wbc = _canonicalize_tsv_line_text("白细胞 6.21 1049/L 3.5-9.5", strategy)
    rbc = _canonicalize_tsv_line_text("红细胞 4.82 10412/L 4.3-5.8", strategy)
    assert _measurement_signature(wbc) == ("6.21", "10^9/L", "3.5-9.5")
    assert _measurement_signature(rbc) == ("4.82", "10^12/L", "4.3-5.8")

    old = _canonicalize_tsv_line_text(
        "白细胞 6.21 1049/L 3.5-9.5",
        "full-ocr-page-tsv-geometry-v3-measurement-rows",
    )
    assert old == "白细胞 6.21 1049/L 3.5-9.5"


def test_v6_percent_unit_normalization_is_bounded_to_scientific_unit_token() -> None:
    strategy = "full-ocr-page-tsv-geometry-v6-percent-unit-normalization"

    rbc = _canonicalize_tsv_line_text("红细胞 4.82 10%12/L 4.3-5.8", strategy)
    assert _measurement_signature(rbc) == ("4.82", "10^12/L", "4.3-5.8")

    numeric_evidence = _canonicalize_tsv_line_text("尿酸 398 umol/L 208-428", strategy)
    assert _measurement_signature(numeric_evidence) == ("398", "umol/L", "208-428")

    old = _canonicalize_tsv_line_text(
        "红细胞 4.82 10%12/L 4.3-5.8",
        "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization",
    )
    assert old == "红细胞 4.82 10%12/L 4.3-5.8"
