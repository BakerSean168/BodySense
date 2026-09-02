"""Resource-bounded RapidOCR + PP-OCRv6-small benchmark candidate.

This is a distinct immutable candidate from ``rapidocr-ppocrv6-small-v1``.
The initial candidate preserved BodySense's historical 300-DPI PDF raster path
and RapidOCR's default detector image-size policy; it failed the 384-MiB
production-shaped resource gate. This candidate deliberately changes both
behavior-significant inputs:

- PDF rasterization is bounded to 150 DPI; and
- PP-OCRv6 detection uses ``limit_type=max`` with long-side limit 736.

The current regex indicator parser remains unchanged so the result still
measures document extraction/OCR quality rather than a parser rewrite.
"""

from __future__ import annotations

import gc
import hashlib
import json
import time
from functools import lru_cache
from importlib import metadata
from pathlib import Path
from typing import Any

import fitz

from ...services.indicator_extractor import extract_indicators
from ..baseline import CandidateIdentityMismatchError, CandidateUnavailableError, sha256_file
from ..contracts import (
    BenchmarkCandidateConfig,
    CorpusDocument,
    FixtureBenchmarkResult,
    PredictedIndicator,
)

SERVICE_ROOT = Path(__file__).resolve().parents[3]
BENCHMARK_ROOT = SERVICE_ROOT / "benchmarks" / "health_document"
RAPIDOCR_BOUNDED_CONFIG_PATH = (
    BENCHMARK_ROOT / "candidates" / "rapidocr-ppocrv6-small-bounded-v1.json"
)

_EXPECTED_ENGINE_PARAMETERS = {
    "Det.engine_type": "onnxruntime",
    "Det.lang_type": "ch",
    "Det.model_type": "small",
    "Det.ocr_version": "PP-OCRv6",
    "Det.limit_type": "max",
    "Det.limit_side_len": "736",
    "Rec.engine_type": "onnxruntime",
    "Rec.lang_type": "ch",
    "Rec.model_type": "small",
    "Rec.ocr_version": "PP-OCRv6",
    "Cls.engine_type": "onnxruntime",
    "Cls.lang_type": "ch",
    "Cls.model_type": "mobile",
    "Cls.ocr_version": "PP-OCRv4",
}


def _assert_equal(label: str, actual: str, expected: str) -> None:
    if actual != expected:
        raise CandidateIdentityMismatchError(f"{label} mismatch: got {actual!r} want {expected!r}")


@lru_cache(maxsize=1)
def load_rapidocr_bounded_config() -> BenchmarkCandidateConfig:
    payload = json.loads(RAPIDOCR_BOUNDED_CONFIG_PATH.read_text(encoding="utf-8"))
    return BenchmarkCandidateConfig.model_validate(payload)


def _rapidocr_models_root() -> Path:
    try:
        import rapidocr  # pyright: ignore[reportMissingImports]
    except ImportError as exc:
        raise CandidateUnavailableError(
            "RapidOCR candidate requires the ocr-benchmark optional dependency"
        ) from exc
    return Path(rapidocr.__file__).resolve().parent / "models"


def verify_rapidocr_bounded_runtime(
    config: BenchmarkCandidateConfig | None = None,
) -> dict[str, str]:
    config = config or load_rapidocr_bounded_config()
    _assert_equal("rapidocr version", metadata.version("rapidocr"), config.engine_version)
    if config.runtime_version is None:
        raise CandidateIdentityMismatchError("RapidOCR config is missing runtime_version")
    _assert_equal("onnxruntime version", metadata.version("onnxruntime"), config.runtime_version)
    _assert_equal("PyMuPDF version", metadata.version("PyMuPDF"), config.pdf_raster.engine_version)
    _assert_equal("Pillow version", metadata.version("Pillow"), config.pillow_version)
    if config.pdf_raster.dpi != 150:
        raise CandidateIdentityMismatchError(
            f"bounded RapidOCR PDF DPI mismatch: {config.pdf_raster.dpi}"
        )
    if config.engine_parameters != _EXPECTED_ENGINE_PARAMETERS:
        raise CandidateIdentityMismatchError("bounded RapidOCR engine parameter contract mismatch")

    expected_adapter = config.source_contract.engine_adapter_sha256
    if expected_adapter is None:
        raise CandidateIdentityMismatchError("RapidOCR config is missing engine_adapter_sha256")
    _assert_equal("bounded RapidOCR adapter sha256", sha256_file(Path(__file__)), expected_adapter)
    _assert_equal(
        "indicator extractor sha256",
        sha256_file(SERVICE_ROOT / "src" / "services" / "indicator_extractor.py"),
        config.source_contract.indicator_extractor_sha256,
    )
    _assert_equal(
        "admissibility policy sha256",
        sha256_file(SERVICE_ROOT / "src" / "services" / "report_indicator_admissibility.py"),
        config.source_contract.admissibility_policy_sha256,
    )

    models_root = _rapidocr_models_root()
    if not config.model_artifacts:
        raise CandidateIdentityMismatchError("RapidOCR config has no model artifacts")
    verified_models: list[str] = []
    for artifact in config.model_artifacts:
        path = models_root / artifact.filename
        if not path.is_file():
            raise CandidateUnavailableError(f"RapidOCR model artifact missing: {path}")
        _assert_equal(f"{artifact.role} model sha256", sha256_file(path), artifact.sha256)
        verified_models.append(f"{artifact.role}:{artifact.sha256}")

    return {
        "status": "verified",
        "candidate_id": config.candidate_id,
        "configuration_id": config.configuration_id,
        "fingerprint": config.fingerprint,
        "engine": config.engine,
        "engine_version": config.engine_version,
        "runtime_engine": config.runtime_engine or "",
        "runtime_version": config.runtime_version,
        "pdf_raster_dpi": str(config.pdf_raster.dpi),
        "detector_limit": "max:736",
        "models": ",".join(verified_models),
    }


@lru_cache(maxsize=1)
def _engine() -> Any:
    config = load_rapidocr_bounded_config()
    verify_rapidocr_bounded_runtime(config)
    try:
        from rapidocr import (  # pyright: ignore[reportMissingImports]
            EngineType,
            LangDet,
            LangRec,
            ModelType,
            OCRVersion,
            RapidOCR,
        )
    except ImportError as exc:
        raise CandidateUnavailableError("RapidOCR package is unavailable") from exc

    return RapidOCR(
        params={
            "Det.engine_type": EngineType.ONNXRUNTIME,
            "Det.lang_type": LangDet.CH,
            "Det.model_type": ModelType.SMALL,
            "Det.ocr_version": OCRVersion.PPOCRV6,
            "Det.limit_type": "max",
            "Det.limit_side_len": 736,
            "Rec.engine_type": EngineType.ONNXRUNTIME,
            "Rec.lang_type": LangRec.CH,
            "Rec.model_type": ModelType.SMALL,
            "Rec.ocr_version": OCRVersion.PPOCRV6,
            "Cls.engine_type": EngineType.ONNXRUNTIME,
            "Cls.lang_type": LangDet.CH,
            "Cls.model_type": ModelType.MOBILE,
            "Cls.ocr_version": OCRVersion.PPOCRV4,
        }
    )


def _ocr_image_bytes(image_bytes: bytes) -> str:
    result = _engine()(image_bytes)
    try:
        return "\n".join(str(text) for text in result.txts if str(text).strip())
    finally:
        del result


def extract_text_rapidocr_bounded(file_bytes: bytes, mime_type: str) -> str:
    config = load_rapidocr_bounded_config()
    if mime_type == "application/pdf":
        doc = fitz.open(stream=file_bytes, filetype="pdf")
        try:
            pages: list[str] = []
            for page_index in range(len(doc)):
                page = doc.load_page(page_index)
                pix = page.get_pixmap(dpi=config.pdf_raster.dpi)
                image_bytes = pix.tobytes(config.pdf_raster.image_format)
                del pix
                text = _ocr_image_bytes(image_bytes)
                del image_bytes
                if text:
                    pages.append(f"--- Page {page_index + 1} ---\n{text}")
                gc.collect()
            return "\n\n".join(pages)
        finally:
            doc.close()
    if mime_type.startswith("image/"):
        return _ocr_image_bytes(file_bytes)
    raise ValueError(f"unsupported benchmark mime_type: {mime_type}")


def run_rapidocr_bounded_fixture(
    document: CorpusDocument,
    *,
    asset_root: Path,
    verify_identity: bool = True,
) -> FixtureBenchmarkResult:
    config = load_rapidocr_bounded_config()
    if verify_identity:
        verify_rapidocr_bounded_runtime(config)
    asset_path = (asset_root / document.asset_path).resolve()
    started = time.perf_counter()
    raw_text = extract_text_rapidocr_bounded(asset_path.read_bytes(), document.mime_type)
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
