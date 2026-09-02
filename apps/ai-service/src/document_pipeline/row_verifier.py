"""Independent Tesseract verifier for OCR-derived health-document pages.

The verifier emits privacy-bounded structured indicators and measurement-row
geometry. It never becomes the source of truth and never edits primary OCR
values. Geometry exists only to align an independently read row when the
secondary engine misreads the Chinese indicator name.
"""

from __future__ import annotations

import csv
import hashlib
import io
import re
import shutil
import subprocess
import unicodedata
from collections import defaultdict
from pathlib import Path

import fitz

from ..configuration.health_document_config import HealthDocumentManifest
from ..models.ocr import (
    HealthDocumentVerifierIndicator,
    HealthDocumentVerifierResponse,
    HealthDocumentVerifierRow,
)
from .baseline import _tessdata_dir, _tesseract_version, sha256_file
from .structured_indicator_parser import SourceTextBlock, extract_structured_indicators

_VALUE_RE = r"[-+]?\d+(?:\.\d+)?"
_UNIT_RE = r"(?:10\s*[\^°]\s*\d+\s*/\s*[A-Za-z]+|[A-Za-zµμ%]+(?:\s*/\s*[A-Za-z0-9µμ^]+)?)"
_RANGE_RE = rf"{_VALUE_RE}\s*[-~～—–]\s*{_VALUE_RE}"
_REFERENCE_LABEL_RE = r"(?:(?:参考\s*范围|参考值|reference\s*range)\s*[:：]?\s*)?"
_MEASUREMENT_WITH_RANGE_RE = re.compile(
    rf"(?P<value>{_VALUE_RE})\s+(?P<unit>{_UNIT_RE})\s+{_REFERENCE_LABEL_RE}(?P<range>{_RANGE_RE})\s*$",
    re.IGNORECASE,
)
_MEASUREMENT_RE = re.compile(
    rf"(?P<value>{_VALUE_RE})\s+(?P<unit>{_UNIT_RE})\s*$",
    re.IGNORECASE,
)
_DASH_TRANSLATION = str.maketrans(
    {"‐": "-", "‑": "-", "‒": "-", "–": "-", "—": "-", "−": "-", "～": "-", "~": "-"}
)


def verifier_adapter_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def verify_row_verifier_runtime(config: HealthDocumentManifest) -> None:
    if config.verification_revision is None:
        raise RuntimeError("health-document verifier is not configured")
    if config.verifier_strategy_revision not in {
        "full-ocr-page-tsv-geometry-v1",
        "full-ocr-page-tsv-geometry-v2-cjk-row-normalization",
        "full-ocr-page-tsv-geometry-v3-measurement-rows",
        "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization",
        "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization",
        "full-ocr-page-tsv-geometry-v6-percent-unit-normalization",
    }:
        raise RuntimeError("unsupported health-document verifier strategy")
    executable = shutil.which("tesseract")
    if executable is None:
        raise RuntimeError("tesseract verifier is unavailable")
    if _tesseract_version() != config.verifier_engine_version:
        raise RuntimeError("tesseract verifier version mismatch")
    tessdata = _tessdata_dir()
    artifacts = {item.language: item for item in config.verifier_language_artifacts or []}
    for language in config.verifier_languages or []:
        artifact = artifacts.get(language)
        if artifact is None:
            raise RuntimeError(f"missing verifier language identity: {language}")
        path = tessdata / f"{language}.traineddata"
        if not path.is_file() or sha256_file(path) != artifact.sha256:
            raise RuntimeError(f"verifier language artifact mismatch: {language}")
    if verifier_adapter_sha256() != config.verifier_adapter_sha256:
        raise RuntimeError("health-document verifier adapter source identity mismatch")
    from .verifier_worker import verifier_worker_source_sha256

    if verifier_worker_source_sha256() != config.verifier_worker_sha256:
        raise RuntimeError("health-document verifier worker source identity mismatch")


def _run_tesseract_tsv(image_bytes: bytes, config: HealthDocumentManifest) -> str:
    executable = shutil.which("tesseract")
    if executable is None:
        raise RuntimeError("tesseract verifier is unavailable")
    proc = subprocess.run(
        [
            executable,
            "stdin",
            "stdout",
            "-l",
            "+".join(config.verifier_languages or []),
            "--psm",
            str(config.verifier_psm),
            "tsv",
        ],
        input=image_bytes,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"tesseract verifier failed with exit code {proc.returncode}")
    return proc.stdout.decode("utf-8", "replace")


def _normalize(value: str) -> str:
    return unicodedata.normalize("NFKC", value).translate(_DASH_TRANSLATION)


def _canonicalize_tsv_line_text(value: str, strategy_revision: str) -> str:
    """Normalize verifier tokenization without altering numeric evidence.

    Tesseract can split one Chinese lexical token into multiple words in TSV
    output (for example ``参考 范围`` or ``甘油 三 酯``). The v2 strategy
    removes whitespace only when both adjacent characters are CJK. Digits,
    decimal points, units and range values are intentionally untouched.
    """

    normalized = _normalize(value)
    if strategy_revision in {
        "full-ocr-page-tsv-geometry-v2-cjk-row-normalization",
        "full-ocr-page-tsv-geometry-v3-measurement-rows",
        "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization",
        "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization",
        "full-ocr-page-tsv-geometry-v6-percent-unit-normalization",
    }:
        normalized = re.sub(r"(?<=[\u3400-\u9fff])\s+(?=[\u3400-\u9fff])", "", normalized)
    if strategy_revision in {
        "full-ocr-page-tsv-geometry-v4-scientific-unit-normalization",
        "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization",
        "full-ocr-page-tsv-geometry-v6-percent-unit-normalization",
    }:
        normalized = re.sub(
            r"(?<=\s)10°(?=\d{1,2}\s*/\s*[A-Za-z]+(?:\s|$))",
            "10^",
            normalized,
        )
    if strategy_revision in {
        "full-ocr-page-tsv-geometry-v5-scientific-unit-ocr-normalization",
        "full-ocr-page-tsv-geometry-v6-percent-unit-normalization",
    }:
        normalized = re.sub(
            r"(?<=\s)104(?P<exponent>\d{1,2})\s*/\s*(?P<denominator>[A-Za-z]+)(?=\s|$)",
            lambda match: f"10^{match.group('exponent')}/{match.group('denominator')}",
            normalized,
        )
    if strategy_revision == "full-ocr-page-tsv-geometry-v6-percent-unit-normalization":
        normalized = re.sub(
            r"(?<=\s)10%(?=\d{1,2}\s*/\s*[A-Za-z]+(?:\s|$))",
            "10^",
            normalized,
        )
    return normalized


def _normalize_verifier_unit(value: str | None) -> str | None:
    if value is None:
        return None
    return re.sub(r"\s+", "", value)


def _measurement_signature(text: str) -> tuple[str, str | None, str | None] | None:
    normalized = _normalize(text).strip()
    match = _MEASUREMENT_WITH_RANGE_RE.search(normalized)
    if match is not None:
        return (
            match.group("value"),
            _normalize_verifier_unit(match.group("unit")),
            match.group("range").replace(" ", ""),
        )
    match = _MEASUREMENT_RE.search(normalized)
    if match is not None:
        return match.group("value"), _normalize_verifier_unit(match.group("unit")), None
    return None


def _parse_tsv_page(
    tsv: str,
    page: int,
    *,
    strategy_revision: str,
) -> tuple[list[HealthDocumentVerifierIndicator], list[HealthDocumentVerifierRow]]:
    grouped: dict[tuple[str, str, str], list[dict[str, str]]] = defaultdict(list)
    for row in csv.DictReader(io.StringIO(tsv), delimiter="\t"):
        if row.get("level") != "5" or not row.get("text", "").strip():
            continue
        grouped[(row.get("block_num", ""), row.get("par_num", ""), row.get("line_num", ""))].append(
            row
        )

    indicators: list[HealthDocumentVerifierIndicator] = []
    verifier_rows: list[HealthDocumentVerifierRow] = []
    for index, items in enumerate(grouped.values(), start=1):
        ordered = sorted(items, key=lambda item: int(item.get("left", "0") or 0))
        text = " ".join(
            item.get("text", "").strip() for item in ordered if item.get("text", "").strip()
        )
        if not text:
            continue
        text = _canonicalize_tsv_line_text(text, strategy_revision)
        top = min(int(item.get("top", "0") or 0) for item in ordered)
        bottom = max(
            int(item.get("top", "0") or 0) + int(item.get("height", "0") or 0) for item in ordered
        )
        signature = _measurement_signature(text)
        if signature is not None:
            value, unit, reference_range = signature
            verifier_rows.append(
                HealthDocumentVerifierRow(
                    page=page,
                    y_center=(top + bottom) / 2.0,
                    height=max(1.0, float(bottom - top)),
                    value=value,
                    unit=unit,
                    reference_range=reference_range,
                )
            )
        parsed = extract_structured_indicators(
            [
                SourceTextBlock(
                    source_ref=f"verifier:page:{page}:line:{index}",
                    page=page,
                    text=text,
                )
            ]
        )
        indicators.extend(
            HealthDocumentVerifierIndicator(
                indicator_id=item.indicator_id or item.name,
                page=page,
                value=item.value,
                unit=item.unit,
                reference_range=item.reference_range,
            )
            for item in parsed
            if item.indicator_id
        )
    return indicators, verifier_rows


def verify_document_pages(
    file_bytes: bytes,
    mime_type: str,
    config: HealthDocumentManifest,
    pages: list[int],
) -> HealthDocumentVerifierResponse:
    verify_row_verifier_runtime(config)
    requested = sorted(set(pages))
    indicators: list[HealthDocumentVerifierIndicator] = []
    rows: list[HealthDocumentVerifierRow] = []
    if not requested:
        return HealthDocumentVerifierResponse(
            configuration_id=config.configuration_id,
            verification_revision=config.verification_revision or "",
        )

    if mime_type == "application/pdf":
        doc = fitz.open(stream=file_bytes, filetype="pdf")
        try:
            for page_number in requested:
                if page_number < 1 or page_number > len(doc):
                    raise ValueError(f"verification page out of range: {page_number}")
                pix = doc.load_page(page_number - 1).get_pixmap(dpi=config.pdf_raster_dpi)
                page_indicators, page_rows = _parse_tsv_page(
                    _run_tesseract_tsv(pix.tobytes("png"), config),
                    page_number,
                    strategy_revision=config.verifier_strategy_revision or "",
                )
                indicators.extend(page_indicators)
                rows.extend(page_rows)
        finally:
            doc.close()
    elif mime_type.startswith("image/"):
        if requested != [1]:
            raise ValueError("image verification only supports page 1")
        indicators, rows = _parse_tsv_page(
            _run_tesseract_tsv(file_bytes, config),
            1,
            strategy_revision=config.verifier_strategy_revision or "",
        )
    else:
        raise ValueError(f"unsupported health-document mime_type: {mime_type}")

    return HealthDocumentVerifierResponse(
        configuration_id=config.configuration_id,
        verification_revision=config.verification_revision or "",
        indicators=indicators,
        rows=rows,
    )
