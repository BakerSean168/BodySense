"""RapidOCR + PP-OCRv6-medium benchmark candidate for constrained CPU runtime.

The candidate keeps the same document behavior variables as the bounded small
candidate (150-DPI PDF raster, detector max-side 736) but replaces the PP-OCRv6
small det/rec models with the official medium artifacts. All model files are
provided from a read-only benchmark model bundle and verified by SHA256 before
execution; runtime downloads are not allowed.
"""

from __future__ import annotations

import gc
import hashlib
import json
import os
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
CONFIG_PATH = BENCHMARK_ROOT / "candidates" / "rapidocr-ppocrv6-medium-v1.json"
MODEL_ROOT_ENV = "BODYSENSE_HEALTH_DOCUMENT_MODEL_ROOT"
DEFAULT_MODEL_ROOT = (
    SERVICE_ROOT
    / "data"
    / "benchmarks"
    / "health_document"
    / "models"
    / "rapidocr-3.9.2"
    / "ppocrv6-medium"
)

_EXPECTED_ENGINE_PARAMETERS = {
    "Det.engine_type": "onnxruntime",
    "Det.lang_type": "ch",
    "Det.model_type": "medium",
    "Det.ocr_version": "PP-OCRv6",
    "Det.limit_type": "max",
    "Det.limit_side_len": "736",
    "Rec.engine_type": "onnxruntime",
    "Rec.lang_type": "ch",
    "Rec.model_type": "medium",
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
def load_rapidocr_medium_config() -> BenchmarkCandidateConfig:
    payload = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))
    return BenchmarkCandidateConfig.model_validate(payload)


def _model_root() -> Path:
    return Path(os.getenv(MODEL_ROOT_ENV, str(DEFAULT_MODEL_ROOT))).resolve()


def verify_rapidocr_medium_runtime(
    config: BenchmarkCandidateConfig | None = None,
) -> dict[str, str]:
    config = config or load_rapidocr_medium_config()
    _assert_equal("rapidocr version", metadata.version("rapidocr"), config.engine_version)
    if config.runtime_version is None:
        raise CandidateIdentityMismatchError("RapidOCR config is missing runtime_version")
    _assert_equal("onnxruntime version", metadata.version("onnxruntime"), config.runtime_version)
    _assert_equal("PyMuPDF version", metadata.version("PyMuPDF"), config.pdf_raster.engine_version)
    _assert_equal("Pillow version", metadata.version("Pillow"), config.pillow_version)
    if config.pdf_raster.dpi != 150:
        raise CandidateIdentityMismatchError(
            f"medium RapidOCR PDF DPI mismatch: {config.pdf_raster.dpi}"
        )
    if config.engine_parameters != _EXPECTED_ENGINE_PARAMETERS:
        raise CandidateIdentityMismatchError("medium RapidOCR engine parameter contract mismatch")

    expected_adapter = config.source_contract.engine_adapter_sha256
    if expected_adapter is None:
        raise CandidateIdentityMismatchError(
            "RapidOCR medium config is missing engine_adapter_sha256"
        )
    _assert_equal("RapidOCR medium adapter sha256", sha256_file(Path(__file__)), expected_adapter)
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

    model_root = _model_root()
    if not config.model_artifacts:
        raise CandidateIdentityMismatchError("RapidOCR medium config has no model artifacts")
    verified: list[str] = []
    for artifact in config.model_artifacts:
        path = model_root / artifact.filename
        if not path.is_file():
            raise CandidateUnavailableError(f"RapidOCR medium model artifact missing: {path}")
        _assert_equal(f"{artifact.role} model sha256", sha256_file(path), artifact.sha256)
        verified.append(f"{artifact.role}:{artifact.sha256}")

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
        "models": ",".join(verified),
    }


@lru_cache(maxsize=1)
def _engine() -> Any:
    config = load_rapidocr_medium_config()
    verify_rapidocr_medium_runtime(config)
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

    by_role = {artifact.role: artifact for artifact in config.model_artifacts or []}
    root = _model_root()
    return RapidOCR(
        params={
            "Det.engine_type": EngineType.ONNXRUNTIME,
            "Det.lang_type": LangDet.CH,
            "Det.model_type": ModelType.MEDIUM,
            "Det.ocr_version": OCRVersion.PPOCRV6,
            "Det.model_path": str(root / by_role["det"].filename),
            "Det.limit_type": "max",
            "Det.limit_side_len": 736,
            "Rec.engine_type": EngineType.ONNXRUNTIME,
            "Rec.lang_type": LangRec.CH,
            "Rec.model_type": ModelType.MEDIUM,
            "Rec.ocr_version": OCRVersion.PPOCRV6,
            "Rec.model_path": str(root / by_role["rec"].filename),
            "Cls.engine_type": EngineType.ONNXRUNTIME,
            "Cls.lang_type": LangDet.CH,
            "Cls.model_type": ModelType.MOBILE,
            "Cls.ocr_version": OCRVersion.PPOCRV4,
            "Cls.model_path": str(root / by_role["cls"].filename),
        }
    )


def _ocr_image_bytes(image_bytes: bytes) -> str:
    result = _engine()(image_bytes)
    try:
        return "\n".join(str(text) for text in result.txts if str(text).strip())
    finally:
        del result


def extract_text_rapidocr_medium(file_bytes: bytes, mime_type: str) -> str:
    config = load_rapidocr_medium_config()
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


def run_rapidocr_medium_fixture(
    document: CorpusDocument,
    *,
    asset_root: Path,
    verify_identity: bool = True,
) -> FixtureBenchmarkResult:
    config = load_rapidocr_medium_config()
    if verify_identity:
        verify_rapidocr_medium_runtime(config)
    asset_path = (asset_root / document.asset_path).resolve()
    started = time.perf_counter()
    raw_text = extract_text_rapidocr_medium(asset_path.read_bytes(), document.mime_type)
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
