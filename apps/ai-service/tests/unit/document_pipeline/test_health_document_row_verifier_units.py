from src.document_pipeline.row_verifier import _canonicalize_tsv_line_text

V3 = "full-ocr-page-tsv-geometry-v3-measurement-rows"
V4 = "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization"
V5 = "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization"


def test_v4_canonicalizes_observed_degree_sign_exponent_alias() -> None:
    source = "白细胞 6.21 10°9/L 3.5-9.5"
    expected = "白细胞 6.21 10^9/L 3.5-9.5"
    assert _canonicalize_tsv_line_text(source, V4) == expected


def test_v5_canonicalizes_observed_digit_four_exponent_aliases() -> None:
    cases = {
        "白细胞 6.21 1049/L 3.5-9.5": "白细胞 6.21 10^9/L 3.5-9.5",
        "红细胞 4.82 10412/L 4.3-5.8": "红细胞 4.82 10^12/L 4.3-5.8",
    }
    for source, expected in cases.items():
        assert _canonicalize_tsv_line_text(source, V5) == expected


def test_scientific_unit_normalization_never_rewrites_measurement_digits() -> None:
    source = "尿酸 398 umol/L 208-428"
    assert _canonicalize_tsv_line_text(source, V5) == source


def test_unapproved_percent_alias_is_not_silently_normalized() -> None:
    source = "红细胞 4.82 10%12/L 4.3-5.8"
    assert _canonicalize_tsv_line_text(source, V5) == source


def test_v3_does_not_inherit_later_scientific_unit_normalization() -> None:
    source = "白细胞 6.21 1049/L 3.5-9.5"
    assert _canonicalize_tsv_line_text(source, V3) == source
