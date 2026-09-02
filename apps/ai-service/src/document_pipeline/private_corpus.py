"""Private real-layout benchmark corpus curation.

This module manages only deidentified evaluation fixtures stored outside Git.
It never attempts to decide whether a document is deidentified. Import and
review operations require an explicit human attestation from the caller.
"""

from __future__ import annotations

import hashlib
import json
import shutil
from pathlib import Path
from typing import Iterable, Literal

import fitz

from .contracts import (
    CorpusCohort,
    CorpusDocument,
    CorpusIndicatorTruth,
    CorpusReviewAttestation,
)
from .corpus import (
    MAX_BENCHMARK_DOCUMENT_BYTES,
    load_corpus_manifest,
    validate_real_layout_selection_subset,
    write_manifest,
)

DEFAULT_PRIVATE_CORPUS_ROOT = (
    Path(__file__).resolve().parents[2]
    / "data"
    / "benchmarks"
    / "health_document"
    / "real-layout-v1"
)

PrivateFixtureMime = Literal["application/pdf", "image/png", "image/jpeg"]

_ALLOWED_SUFFIXES: dict[str, PrivateFixtureMime] = {
    ".pdf": "application/pdf",
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
}


class PrivateCorpusError(ValueError):
    pass


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _manifest_path(root: Path) -> Path:
    return root / "corpus_manifest.jsonl"


def _ensure_private_root(root: Path) -> Path:
    root = root.expanduser().resolve()
    service_root = Path(__file__).resolve().parents[2]
    try:
        relative = root.relative_to(service_root)
    except ValueError:
        return root
    allowed_prefix = Path("data") / "benchmarks" / "health_document"
    if not relative.is_relative_to(allowed_prefix):
        raise PrivateCorpusError(
            "private health-document corpus inside the repository must live under "
            "apps/ai-service/data/benchmarks/health_document (gitignored)"
        )
    return root


def load_private_records(root: Path = DEFAULT_PRIVATE_CORPUS_ROOT) -> list[CorpusDocument]:
    root = _ensure_private_root(root)
    manifest = _manifest_path(root)
    if not manifest.exists():
        return []
    return load_corpus_manifest(manifest)


def _save_records(root: Path, records: Iterable[CorpusDocument]) -> None:
    root = _ensure_private_root(root)
    root.mkdir(parents=True, exist_ok=True)
    write_manifest(_manifest_path(root), sorted(records, key=lambda item: item.fixture_id))


def load_truth(path: Path) -> list[CorpusIndicatorTruth]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(payload, dict):
        payload = payload.get("indicators")
    if not isinstance(payload, list) or not payload:
        raise PrivateCorpusError("truth JSON must contain a non-empty indicator list")
    return [CorpusIndicatorTruth.model_validate(item) for item in payload]


def _page_count(asset: Path, mime_type: PrivateFixtureMime) -> int:
    if mime_type != "application/pdf":
        return 1
    doc = fitz.open(asset)
    try:
        if len(doc) < 1:
            raise PrivateCorpusError("PDF fixture has no pages")
        return len(doc)
    finally:
        doc.close()


def import_deidentified_fixture(
    *,
    source_asset: Path,
    truth: list[CorpusIndicatorTruth],
    fixture_id: str,
    cohort: CorpusCohort,
    human_attests_deidentified: bool,
    root: Path = DEFAULT_PRIVATE_CORPUS_ROOT,
    source_license: str = "private-deidentified-evaluation-only",
) -> CorpusDocument:
    if not human_attests_deidentified:
        raise PrivateCorpusError("import requires explicit human deidentification attestation")
    root = _ensure_private_root(root)
    source_asset = source_asset.expanduser().resolve()
    if not source_asset.is_file():
        raise PrivateCorpusError(f"source asset does not exist: {source_asset}")
    suffix = source_asset.suffix.lower()
    mime_type = _ALLOWED_SUFFIXES.get(suffix)
    if mime_type is None:
        raise PrivateCorpusError("private fixture must be PDF, PNG, JPG, or JPEG")
    size = source_asset.stat().st_size
    if size > MAX_BENCHMARK_DOCUMENT_BYTES:
        raise PrivateCorpusError(
            f"private fixture exceeds product upload limit: {size}>{MAX_BENCHMARK_DOCUMENT_BYTES}"
        )
    existing = load_private_records(root)
    if any(record.fixture_id == fixture_id for record in existing):
        raise PrivateCorpusError(f"fixture already exists: {fixture_id}")

    assets = root / "assets"
    assets.mkdir(parents=True, exist_ok=True)
    target = assets / f"{fixture_id}{suffix}"
    if target.exists():
        raise PrivateCorpusError(f"private fixture asset already exists: {target.name}")
    shutil.copyfile(source_asset, target)
    asset_sha = sha256_file(target)
    record = CorpusDocument(
        schema_version="health-document-corpus-item-v1",
        fixture_id=fixture_id,
        cohort=cohort,
        asset_path=str(Path("assets") / target.name),
        mime_type=mime_type,
        page_count=_page_count(target, mime_type),
        source_classification="deidentified",
        contains_real_user_data=False,
        source_license=source_license,
        asset_sha256=asset_sha,
        annotation_state="unreviewed",
        indicators=truth,
    )
    _save_records(root, [*existing, record])
    return record


def _replace_record(records: list[CorpusDocument], updated: CorpusDocument) -> list[CorpusDocument]:
    found = False
    output: list[CorpusDocument] = []
    for record in records:
        if record.fixture_id == updated.fixture_id:
            output.append(updated)
            found = True
        else:
            output.append(record)
    if not found:
        raise PrivateCorpusError(f"unknown private fixture: {updated.fixture_id}")
    return output


def set_fixture_truth(
    *,
    fixture_id: str,
    truth: list[CorpusIndicatorTruth],
    root: Path = DEFAULT_PRIVATE_CORPUS_ROOT,
) -> CorpusDocument:
    records = load_private_records(root)
    current = next((record for record in records if record.fixture_id == fixture_id), None)
    if current is None:
        raise PrivateCorpusError(f"unknown private fixture: {fixture_id}")
    payload = current.model_dump(mode="json")
    payload["indicators"] = [item.model_dump(mode="json") for item in truth]
    payload["annotation_state"] = "unreviewed"
    payload["review_attestations"] = []
    updated = CorpusDocument.model_validate(payload)
    _save_records(root, _replace_record(records, updated))
    return updated


def review_fixture(
    *,
    fixture_id: str,
    reviewer_id: str,
    human_attests_deidentified: bool,
    root: Path = DEFAULT_PRIVATE_CORPUS_ROOT,
) -> CorpusDocument:
    if not human_attests_deidentified:
        raise PrivateCorpusError("review requires explicit deidentification attestation")
    records = load_private_records(root)
    current = next((record for record in records if record.fixture_id == fixture_id), None)
    if current is None:
        raise PrivateCorpusError(f"unknown private fixture: {fixture_id}")
    if any(item.reviewer_id == reviewer_id for item in current.review_attestations):
        raise PrivateCorpusError(f"reviewer already attested this truth: {reviewer_id}")
    attestation = CorpusReviewAttestation(
        reviewer_id=reviewer_id,
        truth_sha256=current.truth_sha256(),
        deidentification_attested=True,
    )
    attestations = [*current.review_attestations, attestation]
    payload = current.model_dump(mode="json")
    payload["review_attestations"] = [item.model_dump(mode="json") for item in attestations]
    payload["annotation_state"] = "double_reviewed" if len(attestations) >= 2 else "single_reviewed"
    updated = CorpusDocument.model_validate(payload)
    _save_records(root, _replace_record(records, updated))
    return updated


def private_corpus_status(root: Path = DEFAULT_PRIVATE_CORPUS_ROOT) -> dict[str, object]:
    records = load_private_records(root)
    by_state: dict[str, int] = {}
    by_cohort: dict[str, int] = {}
    for record in records:
        by_state[record.annotation_state] = by_state.get(record.annotation_state, 0) + 1
        by_cohort[record.cohort] = by_cohort.get(record.cohort, 0) + 1
    return {
        "documents": len(records),
        "annotation_states": dict(sorted(by_state.items())),
        "cohorts": dict(sorted(by_cohort.items())),
        "manifest_sha256": (
            sha256_file(_manifest_path(_ensure_private_root(root))) if records else None
        ),
    }


def validate_private_selection_corpus(
    root: Path = DEFAULT_PRIVATE_CORPUS_ROOT,
) -> dict[str, object]:
    root = _ensure_private_root(root)
    records = load_private_records(root)
    summary = validate_real_layout_selection_subset(records, asset_root=root)
    return {
        "documents": summary.documents,
        "pages": summary.pages,
        "cohorts": list(summary.cohorts),
        "double_reviewed_documents": summary.double_reviewed_documents,
        "manifest_sha256": sha256_file(_manifest_path(root)),
    }
