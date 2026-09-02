from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import cast

import pytest

from src.document_pipeline.baseline import (
    CandidateIdentityMismatchError,
    load_tesseract_baseline_config,
    verify_tesseract_baseline_source_contract,
)
from src.document_pipeline.contracts import (
    CorpusCohort,
    CorpusDocument,
    CorpusIndicatorTruth,
    CorpusReviewAttestation,
    PredictedIndicator,
)
from src.document_pipeline.corpus import (
    REQUIRED_COHORTS,
    validate_corpus,
    validate_real_layout_selection_subset,
)
from src.document_pipeline.metrics import summarize_accuracy

SERVICE_ROOT = Path(__file__).resolve().parents[3]
CORPUS_SPEC = SERVICE_ROOT / "benchmarks" / "health_document" / "corpus_spec.json"


def test_frozen_tesseract_production_baseline_identity_is_stable() -> None:
    config = load_tesseract_baseline_config()
    assert config.candidate_id == "tesseract-production-v0.10.2"
    assert config.configuration_id == "hdex-config-14af808ef184bf8b"
    assert config.fingerprint == "14af808ef184bf8bbdae215c605408b90cfdd6dcb7242c4afd5818058004aa0d"
    assert config.engine_version == "5.5.0"
    assert config.languages == ["chi_sim", "eng"]
    assert config.pdf_raster.dpi == 300


def test_frozen_tesseract_source_contract_matches_current_historical_implementation() -> None:
    verified = verify_tesseract_baseline_source_contract()
    assert verified["ocr_service_sha256"] == (
        "cd31066a2be9f80e38a00c636f10d31bdf83c5b21165c8084acb9cb7cde60220"
    )
    assert verified["indicator_extractor_sha256"] == (
        "22d6e1c8689557c8b5a878004a647b428950b47c44602c958ce1cb54e383eec7"
    )


def test_source_contract_fails_if_declared_hash_is_forged() -> None:
    config = load_tesseract_baseline_config()
    forged = config.model_copy(
        update={
            "source_contract": config.source_contract.model_copy(
                update={"ocr_service_sha256": "0" * 64}
            )
        }
    )
    with pytest.raises(CandidateIdentityMismatchError, match="ocr_service_sha256 mismatch"):
        verify_tesseract_baseline_source_contract(forged)


def test_synthetic_corpus_spec_is_40_documents_100_pages_and_all_required_cohorts() -> None:
    spec = json.loads(CORPUS_SPEC.read_text(encoding="utf-8"))
    documents = sum(int(cohort["documents"]) for cohort in spec["cohorts"])
    pages = sum(
        int(cohort["documents"]) * int(cohort["pages_per_document"]) for cohort in spec["cohorts"]
    )
    cohorts = {str(cohort["id"]) for cohort in spec["cohorts"]}
    assert documents == 40
    assert pages == 100
    assert cohorts == REQUIRED_COHORTS


def _synthetic_records(tmp_path: Path) -> list[CorpusDocument]:
    spec = json.loads(CORPUS_SPEC.read_text(encoding="utf-8"))
    records: list[CorpusDocument] = []
    for cohort in spec["cohorts"]:
        cohort_id = cast(CorpusCohort, str(cohort["id"]))
        for index in range(int(cohort["documents"])):
            fixture_id = f"{cohort_id.replace('_', '-')}-{index + 1:02d}"
            asset_path = Path("assets") / f"{fixture_id}.pdf"
            absolute = tmp_path / asset_path
            absolute.parent.mkdir(parents=True, exist_ok=True)
            absolute.write_bytes(b"synthetic")
            asset_sha = hashlib.sha256(absolute.read_bytes()).hexdigest()
            records.append(
                CorpusDocument(
                    schema_version="health-document-corpus-item-v1",
                    fixture_id=fixture_id,
                    cohort=cohort_id,
                    asset_path=str(asset_path),
                    mime_type="application/pdf",
                    page_count=int(cohort["pages_per_document"]),
                    source_classification="synthetic",
                    contains_real_user_data=False,
                    source_license="synthetic-test",
                    generator_revision="test-v1",
                    generator_asset_sha256=asset_sha,
                    annotation_state="synthetic_ground_truth",
                    indicators=[],
                )
            )
    return records


def test_synthetic_corpus_passes_mechanics_gate_but_not_champion_selection_gate(
    tmp_path: Path,
) -> None:
    records = _synthetic_records(tmp_path)
    summary = validate_corpus(
        records,
        asset_root=tmp_path,
        require_minimum_shape=True,
    )
    assert summary.documents == 40
    assert summary.pages == 100
    assert summary.deidentified_documents == 0

    with pytest.raises(ValueError, match="at least 10 deidentified real-layout"):
        validate_corpus(
            records,
            asset_root=tmp_path,
            require_selection_ready=True,
        )


def test_corpus_rejects_fixture_larger_than_product_upload_limit(tmp_path: Path) -> None:
    record = _synthetic_records(tmp_path)[0]
    asset = tmp_path / record.asset_path
    asset.write_bytes(b"x" * (10 * 1024 * 1024 + 1))
    with pytest.raises(ValueError, match="exceeds product upload limit"):
        validate_corpus([record], asset_root=tmp_path)


def _real_layout_record(
    tmp_path: Path,
    *,
    fixture_id: str,
    cohort: CorpusCohort,
    reviewers: tuple[str, ...] = ("reviewer-a", "reviewer-b"),
) -> CorpusDocument:
    asset = tmp_path / "assets" / f"{fixture_id}.pdf"
    asset.parent.mkdir(parents=True, exist_ok=True)
    asset.write_bytes(f"deidentified-{fixture_id}".encode())
    truth = [
        CorpusIndicatorTruth(
            indicator_id="hemoglobin",
            display_name="血红蛋白",
            value="142",
            unit="g/L",
            reference_range="130-175",
            page=1,
            row=1,
        )
    ]
    draft = CorpusDocument(
        schema_version="health-document-corpus-item-v1",
        fixture_id=fixture_id,
        cohort=cohort,
        asset_path=str(Path("assets") / asset.name),
        mime_type="application/pdf",
        page_count=1,
        source_classification="deidentified",
        contains_real_user_data=False,
        source_license="private-deidentified-evaluation-only",
        asset_sha256=hashlib.sha256(asset.read_bytes()).hexdigest(),
        annotation_state="unreviewed",
        indicators=truth,
    )
    attestations = [
        CorpusReviewAttestation(
            reviewer_id=reviewer,
            truth_sha256=draft.truth_sha256(),
            deidentification_attested=True,
        )
        for reviewer in reviewers
    ]
    return draft.model_copy(
        update={
            "annotation_state": (
                "double_reviewed"
                if len(attestations) >= 2
                else "single_reviewed"
                if attestations
                else "unreviewed"
            ),
            "review_attestations": attestations,
        }
    )


def test_deidentified_double_review_requires_two_distinct_reviewers_on_exact_truth(
    tmp_path: Path,
) -> None:
    record = _real_layout_record(
        tmp_path, fixture_id="private-native-01", cohort="native_pdf_simple"
    )
    assert record.annotation_state == "double_reviewed"
    assert len(record.review_attestations) == 2

    payload = record.model_dump(mode="json")
    payload["review_attestations"] = [
        record.review_attestations[0].model_dump(mode="json"),
        record.review_attestations[0].model_dump(mode="json"),
    ]
    with pytest.raises(ValueError, match="distinct reviewer_id"):
        CorpusDocument.model_validate(payload)


def test_review_attestation_is_invalidated_when_truth_changes(tmp_path: Path) -> None:
    record = _real_layout_record(
        tmp_path, fixture_id="private-native-02", cohort="native_pdf_simple"
    )
    changed_truth = [record.indicators[0].model_copy(update={"value": "143"})]
    payload = record.model_dump(mode="json")
    payload["indicators"] = [item.model_dump(mode="json") for item in changed_truth]
    with pytest.raises(ValueError, match="truth_sha256 does not match"):
        CorpusDocument.model_validate(payload)


def test_real_layout_selection_subset_requires_ten_double_reviewed_docs_and_all_risk_groups(
    tmp_path: Path,
) -> None:
    cohorts: list[CorpusCohort] = [
        "native_pdf_simple",
        "scanned_pdf_clear",
        "phone_photo_clear",
        "complex_table",
        "native_pdf_table",
        "scanned_pdf_degraded",
        "phone_photo_degraded",
        "chinese_lab_table",
        "mixed_zh_en_units",
        "complex_table",
    ]
    records = [
        _real_layout_record(
            tmp_path,
            fixture_id=f"private-layout-{index:02d}",
            cohort=cohort,
        )
        for index, cohort in enumerate(cohorts, start=1)
    ]
    summary = validate_real_layout_selection_subset(records, asset_root=tmp_path)
    assert summary.deidentified_documents == 10
    assert summary.double_reviewed_documents == 10

    with pytest.raises(ValueError, match="at least 10"):
        validate_real_layout_selection_subset(records[:9], asset_root=tmp_path)

    without_photo = [
        record.model_copy(update={"cohort": "scanned_pdf_clear"})
        if record.cohort.startswith("phone_photo")
        else record
        for record in records
    ]
    with pytest.raises(ValueError, match="missing risk groups: photo"):
        validate_real_layout_selection_subset(without_photo, asset_root=tmp_path)


def _document_with_truth(*truth: CorpusIndicatorTruth) -> CorpusDocument:
    return CorpusDocument(
        schema_version="health-document-corpus-item-v1",
        fixture_id="metric-fixture-01",
        cohort="native_pdf_simple",
        asset_path="metric-fixture.pdf",
        mime_type="application/pdf",
        page_count=1,
        source_classification="synthetic",
        contains_real_user_data=False,
        source_license="synthetic-test",
        generator_revision="test-v1",
        annotation_state="synthetic_ground_truth",
        indicators=list(truth),
    )


def test_missing_indicator_reduces_exact_match_but_is_not_critical_numeric_substitution() -> None:
    document = _document_with_truth(
        CorpusIndicatorTruth(
            indicator_id="hemoglobin",
            display_name="血红蛋白",
            value="142",
            unit="g/L",
            reference_range="130-175",
            page=1,
            row=1,
        )
    )
    summary = summarize_accuracy([document], {document.fixture_id: []})
    assert summary.numeric_exact_match == 0
    assert summary.indicator_exact_match == 0
    assert summary.critical_numeric_error_rate == 0


def test_wrong_numeric_value_counts_as_critical_numeric_error() -> None:
    document = _document_with_truth(
        CorpusIndicatorTruth(
            indicator_id="vitamin_d",
            display_name="维生素D",
            value="17.8",
            unit="ng/mL",
            reference_range="30-100",
            page=1,
            row=1,
        )
    )
    predictions = {
        document.fixture_id: [
            PredictedIndicator(
                name="维生素D",
                value="77.8",
                unit="ng/mL",
                reference_range="30-100",
            )
        ]
    }
    summary = summarize_accuracy([document], predictions)
    assert summary.numeric_exact_match == 0
    assert summary.unit_exact_match == 1
    assert summary.critical_numeric_error_rate == 1


def test_rapidocr_small_candidate_identity_is_stable_and_source_bound() -> None:
    from src.document_pipeline.baseline import sha256_file
    from src.document_pipeline.engines import rapidocr_ppocrv6

    config = rapidocr_ppocrv6.load_rapidocr_small_config()
    assert config.candidate_id == "rapidocr-ppocrv6-small-v1"
    assert config.configuration_id == "hdex-config-381a5312d0eddc5c"
    assert config.engine == "rapidocr"
    assert config.engine_version == "3.9.2"
    assert config.runtime_engine == "onnxruntime"
    assert config.runtime_version == "1.29.0"
    assert config.model_family == "PP-OCRv6"
    assert config.model_type == "small"
    assert config.source_contract.engine_adapter_sha256 == sha256_file(
        Path(rapidocr_ppocrv6.__file__)
    )


def test_rapidocr_model_identity_change_changes_configuration_id() -> None:
    from src.document_pipeline.engines.rapidocr_ppocrv6 import load_rapidocr_small_config

    config = load_rapidocr_small_config()
    assert config.model_artifacts
    changed_artifacts = list(config.model_artifacts)
    changed_artifacts[0] = changed_artifacts[0].model_copy(update={"sha256": "0" * 64})
    changed = config.model_copy(update={"model_artifacts": changed_artifacts})
    assert changed.configuration_id != config.configuration_id


def test_rapidocr_bounded_candidate_identity_is_stable_and_distinct_from_failed_v1() -> None:
    from src.document_pipeline.engines.rapidocr_ppocrv6 import load_rapidocr_small_config
    from src.document_pipeline.engines.rapidocr_ppocrv6_bounded import (
        load_rapidocr_bounded_config,
    )

    failed_v1 = load_rapidocr_small_config()
    bounded = load_rapidocr_bounded_config()
    assert bounded.candidate_id == "rapidocr-ppocrv6-small-bounded-v1"
    assert bounded.configuration_id == "hdex-config-0b014f7a50b2aded"
    assert bounded.configuration_id != failed_v1.configuration_id
    assert bounded.pdf_raster.dpi == 150
    assert bounded.engine_parameters is not None
    assert bounded.engine_parameters["Det.limit_type"] == "max"
    assert bounded.engine_parameters["Det.limit_side_len"] == "736"


def test_rapidocr_tiny_candidate_identity_is_stable_and_model_distinct() -> None:
    from src.document_pipeline.baseline import sha256_file
    from src.document_pipeline.engines import rapidocr_ppocrv6_tiny
    from src.document_pipeline.engines.rapidocr_ppocrv6_bounded import (
        load_rapidocr_bounded_config,
    )

    tiny = rapidocr_ppocrv6_tiny.load_rapidocr_tiny_config()
    small = load_rapidocr_bounded_config()
    assert tiny.candidate_id == "rapidocr-ppocrv6-tiny-v1"
    assert tiny.configuration_id == "hdex-config-f4911e8b12684272"
    assert tiny.configuration_id != small.configuration_id
    assert tiny.model_family == "PP-OCRv6"
    assert tiny.model_type == "tiny"
    assert tiny.pdf_raster.dpi == 150
    assert tiny.engine_parameters is not None
    assert tiny.engine_parameters["Det.limit_type"] == "max"
    assert tiny.engine_parameters["Det.limit_side_len"] == "736"
    assert tiny.source_contract.engine_adapter_sha256 == sha256_file(
        Path(rapidocr_ppocrv6_tiny.__file__)
    )
    assert tiny.model_artifacts
    by_role = {artifact.role: artifact.sha256 for artifact in tiny.model_artifacts}
    assert by_role["det"] == "f42c0fbd294d95eac1a550e131b277dac97462c8025fa4b6c3cec1b7894bd3d5"
    assert by_role["rec"] == "e16e242de5937ad92609223f19bc2aff3727ee40b095f996907c24749bad251b"


def test_source_accuracy_is_not_downgraded_by_legacy_regex_unit_limit() -> None:
    from src.document_pipeline.metrics import evaluate_source_text, summarize_accuracy
    from src.services.indicator_extractor import extract_indicators

    document = CorpusDocument(
        schema_version="health-document-corpus-item-v1",
        fixture_id="source-unit-separation",
        cohort="chinese_lab_table",
        asset_path="fixture.png",
        mime_type="image/png",
        page_count=1,
        source_classification="synthetic",
        contains_real_user_data=False,
        source_license="synthetic",
        generator_revision="test",
        annotation_state="synthetic_ground_truth",
        indicators=[
            CorpusIndicatorTruth(
                indicator_id="wbc",
                display_name="白细胞",
                value="6.21",
                unit="10^9/L",
                reference_range="3.5-9.5",
                page=1,
                row=1,
            )
        ],
    )
    raw_text = "白细胞: 6.21 10^9/L 参考范围: 3.5-9.5"
    source = evaluate_source_text(document, raw_text)
    assert source.numeric_exact == 1
    assert source.unit_exact == 1
    assert source.reference_range_exact == 1
    assert source.row_bundle_exact == 1

    parsed = extract_indicators(raw_text)
    e2e = summarize_accuracy(
        [document],
        {
            document.fixture_id: [
                PredictedIndicator(
                    name=item.name,
                    value=item.value,
                    unit=item.unit,
                    reference_range=item.reference_range,
                    confidence=item.confidence,
                )
                for item in parsed
            ]
        },
    )
    assert e2e.numeric_exact_match == 1
    assert e2e.unit_exact_match == 0


def test_source_accuracy_normalizes_pdf_compatibility_glyphs_and_nonbreaking_hyphens() -> None:
    from src.document_pipeline.metrics import evaluate_source_text

    document = CorpusDocument(
        schema_version="health-document-corpus-item-v1",
        fixture_id="native-unicode-normalization",
        cohort="native_pdf_simple",
        asset_path="fixture.pdf",
        mime_type="application/pdf",
        page_count=1,
        source_classification="synthetic",
        contains_real_user_data=False,
        source_license="synthetic",
        generator_revision="test",
        annotation_state="synthetic_ground_truth",
        indicators=[
            CorpusIndicatorTruth(
                indicator_id="uric_acid",
                display_name="尿酸",
                value="398",
                unit="umol/L",
                reference_range="208-428",
                page=1,
                row=1,
            )
        ],
    )
    source = evaluate_source_text(document, "尿酸: 398 umol/L 参考范围: 208‑428")
    assert source.name_present == 1
    assert source.numeric_exact == 1
    assert source.unit_exact == 1
    assert source.reference_range_exact == 1
    assert source.row_bundle_exact == 1
