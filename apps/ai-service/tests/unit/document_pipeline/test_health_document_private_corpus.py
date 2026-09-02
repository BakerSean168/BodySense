from __future__ import annotations

import json
from pathlib import Path

import fitz
import pytest

from src.document_pipeline.contracts import CorpusIndicatorTruth
from src.document_pipeline.private_corpus import (
    PrivateCorpusError,
    import_deidentified_fixture,
    load_private_records,
    private_corpus_status,
    review_fixture,
    set_fixture_truth,
)


def _pdf(path: Path) -> Path:
    doc = fitz.open()
    try:
        page = doc.new_page()
        page.insert_text((72, 72), "DEIDENTIFIED TEST FIXTURE")
        doc.save(path, no_new_id=True)
    finally:
        doc.close()
    return path


def _truth(value: str = "142") -> list[CorpusIndicatorTruth]:
    return [
        CorpusIndicatorTruth(
            indicator_id="hemoglobin",
            display_name="血红蛋白",
            value=value,
            unit="g/L",
            reference_range="130-175",
            page=1,
            row=1,
        )
    ]


def test_private_fixture_import_requires_explicit_deidentification_attestation(
    tmp_path: Path,
) -> None:
    source = _pdf(tmp_path / "source.pdf")
    root = tmp_path / "private-corpus"
    with pytest.raises(PrivateCorpusError, match="explicit human deidentification"):
        import_deidentified_fixture(
            source_asset=source,
            truth=_truth(),
            fixture_id="private-lab-01",
            cohort="native_pdf_simple",
            human_attests_deidentified=False,
            root=root,
        )


def test_private_fixture_review_lifecycle_is_hash_bound_and_truth_changes_reset_reviews(
    tmp_path: Path,
) -> None:
    source = _pdf(tmp_path / "external-deidentified.pdf")
    root = tmp_path / "private-corpus"
    imported = import_deidentified_fixture(
        source_asset=source,
        truth=_truth(),
        fixture_id="private-lab-01",
        cohort="native_pdf_simple",
        human_attests_deidentified=True,
        root=root,
    )
    assert imported.annotation_state == "unreviewed"
    assert imported.asset_sha256 is not None
    manifest_text = (root / "corpus_manifest.jsonl").read_text(encoding="utf-8")
    assert str(source) not in manifest_text
    assert imported.asset_path == "assets/private-lab-01.pdf"

    first = review_fixture(
        fixture_id=imported.fixture_id,
        reviewer_id="reviewer-a",
        human_attests_deidentified=True,
        root=root,
    )
    assert first.annotation_state == "single_reviewed"
    assert first.review_attestations[0].truth_sha256 == first.truth_sha256()

    second = review_fixture(
        fixture_id=imported.fixture_id,
        reviewer_id="reviewer-b",
        human_attests_deidentified=True,
        root=root,
    )
    assert second.annotation_state == "double_reviewed"
    assert {item.reviewer_id for item in second.review_attestations} == {
        "reviewer-a",
        "reviewer-b",
    }

    with pytest.raises(PrivateCorpusError, match="already attested"):
        review_fixture(
            fixture_id=imported.fixture_id,
            reviewer_id="reviewer-b",
            human_attests_deidentified=True,
            root=root,
        )

    changed = set_fixture_truth(
        fixture_id=imported.fixture_id,
        truth=_truth("143"),
        root=root,
    )
    assert changed.annotation_state == "unreviewed"
    assert changed.review_attestations == []
    assert changed.truth_sha256() != second.truth_sha256()


def test_private_corpus_status_is_privacy_bounded(tmp_path: Path) -> None:
    source = _pdf(tmp_path / "private-source.pdf")
    root = tmp_path / "private-corpus"
    import_deidentified_fixture(
        source_asset=source,
        truth=_truth(),
        fixture_id="private-lab-02",
        cohort="native_pdf_simple",
        human_attests_deidentified=True,
        root=root,
    )
    status = private_corpus_status(root)
    serialized = json.dumps(status, ensure_ascii=False)
    assert status["documents"] == 1
    assert status["annotation_states"] == {"unreviewed": 1}
    assert str(source) not in serialized
    assert "142" not in serialized
    assert len(load_private_records(root)) == 1
