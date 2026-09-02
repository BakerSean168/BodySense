"""Adapter for the frozen BodySense v0.10.2 Tesseract production baseline."""

from __future__ import annotations

import hashlib
import time
from pathlib import Path

from ...services.indicator_extractor import extract_indicators
from ...services.ocr import extract_text
from ..baseline import load_tesseract_baseline_config, verify_tesseract_baseline_runtime
from ..contracts import CorpusDocument, FixtureBenchmarkResult, PredictedIndicator


def run_tesseract_fixture(
    document: CorpusDocument,
    *,
    asset_root: Path,
    verify_identity: bool = True,
) -> FixtureBenchmarkResult:
    if verify_identity:
        verify_tesseract_baseline_runtime(load_tesseract_baseline_config())

    asset_path = (asset_root / document.asset_path).resolve()
    started = time.perf_counter()
    raw_text, _ = extract_text(asset_path.read_bytes(), document.mime_type)
    indicators = extract_indicators(raw_text)
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    return FixtureBenchmarkResult(
        fixture_id=document.fixture_id,
        cohort=document.cohort,
        elapsed_ms=elapsed_ms,
        raw_text_sha256=hashlib.sha256(raw_text.encode()).hexdigest(),
        predicted_indicators=[
            PredictedIndicator(
                name=indicator.name,
                value=indicator.value,
                unit=indicator.unit,
                reference_range=indicator.reference_range,
                confidence=indicator.confidence,
            )
            for indicator in indicators
        ],
    )
