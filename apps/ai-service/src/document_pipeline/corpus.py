"""Health-document benchmark corpus loading and privacy/shape validation."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from pathlib import Path

from .contracts import CorpusDocument

MAX_BENCHMARK_DOCUMENT_BYTES = 10 * 1024 * 1024
MIN_REAL_LAYOUT_DOCUMENTS = 10

REQUIRED_COHORTS = {
    "native_pdf_simple",
    "native_pdf_table",
    "scanned_pdf_clear",
    "scanned_pdf_degraded",
    "phone_photo_clear",
    "phone_photo_degraded",
    "chinese_lab_table",
    "mixed_zh_en_units",
    "complex_table",
}

REAL_LAYOUT_RISK_GROUPS = {
    "native": {"native_pdf_simple", "native_pdf_table", "mixed_zh_en_units"},
    "scanned": {"scanned_pdf_clear", "scanned_pdf_degraded"},
    "photo": {"phone_photo_clear", "phone_photo_degraded"},
    "table": {"chinese_lab_table", "complex_table", "native_pdf_table"},
}


@dataclass(frozen=True)
class CorpusValidationSummary:
    documents: int
    pages: int
    cohorts: tuple[str, ...]
    synthetic_documents: int
    deidentified_documents: int
    double_reviewed_documents: int


def manifest_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_corpus_manifest(path: Path) -> list[CorpusDocument]:
    records: list[CorpusDocument] = []
    for line_no, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            records.append(CorpusDocument.model_validate_json(line))
        except Exception as exc:  # noqa: BLE001
            raise ValueError(f"invalid corpus manifest line {line_no}: {exc}") from exc
    if not records:
        raise ValueError("corpus manifest is empty")
    return records


def validate_corpus(
    records: list[CorpusDocument],
    *,
    asset_root: Path,
    require_minimum_shape: bool = False,
    require_selection_ready: bool = False,
) -> CorpusValidationSummary:
    fixture_ids: set[str] = set()
    cohorts: set[str] = set()
    pages = 0
    synthetic = 0
    deidentified = 0
    double_reviewed = 0

    root = asset_root.resolve()
    for record in records:
        if record.fixture_id in fixture_ids:
            raise ValueError(f"duplicate fixture_id: {record.fixture_id}")
        fixture_ids.add(record.fixture_id)
        cohorts.add(record.cohort)
        pages += record.page_count
        synthetic += int(record.source_classification == "synthetic")
        deidentified += int(record.source_classification == "deidentified")
        double_reviewed += int(record.annotation_state == "double_reviewed")

        relative = Path(record.asset_path)
        if relative.is_absolute() or ".." in relative.parts:
            raise ValueError(
                f"fixture {record.fixture_id} asset_path must be relative and contained"
            )
        resolved = (root / relative).resolve()
        if root not in resolved.parents and resolved != root:
            raise ValueError(f"fixture {record.fixture_id} escapes asset root")
        if not resolved.is_file():
            raise ValueError(f"fixture asset missing: {resolved}")
        size = resolved.stat().st_size
        if size > MAX_BENCHMARK_DOCUMENT_BYTES:
            raise ValueError(
                f"fixture {record.fixture_id} exceeds product upload limit: "
                f"{size}>{MAX_BENCHMARK_DOCUMENT_BYTES}"
            )
        if require_minimum_shape or require_selection_ready:
            actual_sha = hashlib.sha256(resolved.read_bytes()).hexdigest()
            expected_sha = (
                record.generator_asset_sha256
                if record.source_classification == "synthetic"
                else record.asset_sha256
            )
            if expected_sha is None:
                raise ValueError(f"fixture {record.fixture_id} is missing immutable asset sha256")
            if actual_sha != expected_sha:
                raise ValueError(
                    f"fixture {record.fixture_id} asset sha256 mismatch: "
                    f"got {actual_sha} want {expected_sha}"
                )

    if require_minimum_shape or require_selection_ready:
        if len(records) < 40:
            raise ValueError(f"selection corpus requires >=40 documents, got {len(records)}")
        if pages < 100:
            raise ValueError(f"selection corpus requires >=100 pages, got {pages}")
        missing = REQUIRED_COHORTS - cohorts
        if missing:
            raise ValueError(f"selection corpus missing required cohorts: {sorted(missing)}")
    if require_selection_ready:
        real_layout = [
            record for record in records if record.source_classification == "deidentified"
        ]
        _validate_real_layout_records(real_layout)

    return CorpusValidationSummary(
        documents=len(records),
        pages=pages,
        cohorts=tuple(sorted(cohorts)),
        synthetic_documents=synthetic,
        deidentified_documents=deidentified,
        double_reviewed_documents=double_reviewed,
    )


def _validate_real_layout_records(records: list[CorpusDocument]) -> None:
    if len(records) < MIN_REAL_LAYOUT_DOCUMENTS:
        raise ValueError(
            "Champion selection requires at least "
            f"{MIN_REAL_LAYOUT_DOCUMENTS} deidentified real-layout documents; got {len(records)}"
        )
    non_double = [
        record.fixture_id for record in records if record.annotation_state != "double_reviewed"
    ]
    if non_double:
        raise ValueError(
            "Champion selection requires every real-layout fixture to be double_reviewed: "
            + ", ".join(sorted(non_double))
        )
    cohorts = {record.cohort for record in records}
    missing_groups = [
        group
        for group, accepted in REAL_LAYOUT_RISK_GROUPS.items()
        if not cohorts.intersection(accepted)
    ]
    if missing_groups:
        raise ValueError(
            "real-layout selection subset is missing risk groups: " + ", ".join(missing_groups)
        )


def validate_real_layout_selection_subset(
    records: list[CorpusDocument],
    *,
    asset_root: Path,
) -> CorpusValidationSummary:
    if any(record.source_classification != "deidentified" for record in records):
        raise ValueError("real-layout selection subset may contain only deidentified fixtures")
    summary = validate_corpus(records, asset_root=asset_root)
    for record in records:
        resolved = (asset_root.resolve() / Path(record.asset_path)).resolve()
        actual_sha = hashlib.sha256(resolved.read_bytes()).hexdigest()
        if record.asset_sha256 != actual_sha:
            raise ValueError(
                f"fixture {record.fixture_id} asset sha256 mismatch: "
                f"got {actual_sha} want {record.asset_sha256}"
            )
    _validate_real_layout_records(records)
    return summary


def write_manifest(path: Path, records: list[CorpusDocument]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    content = "\n".join(
        json.dumps(record.model_dump(mode="json"), ensure_ascii=False, sort_keys=True)
        for record in records
    )
    path.write_text(content + "\n", encoding="utf-8")
