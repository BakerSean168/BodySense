"""Short-lived health-document extraction worker.

One process handles exactly one document. The process owns RapidOCR/ONNX native
allocations and exits after emitting one validated JSON response on stdout.
Raw document bytes arrive on stdin and are never written to logs or disk.
"""

from __future__ import annotations

import argparse
import hashlib
import sys
from pathlib import Path

from ..configuration.health_document_config import get_health_document_configuration
from ..models.ocr import (
    HealthDocumentMechanismProvenance,
    LegacyTesseractMechanismProvenance,
    OCRConfidence,
    OCRResponse,
    OCRResult,
)
from ..services.indicator_extractor import extract_indicators, get_overall_confidence
from ..services.ocr import extract_text as extract_text_tesseract
from ..services.report_indicator_admissibility import apply_indicator_admissibility
from ..services.report_indicator_admissibility_v1 import (
    apply_indicator_admissibility as apply_indicator_admissibility_v1,
)
from .baseline import load_tesseract_baseline_config, verify_tesseract_baseline_runtime
from .serving_engine import extract_document, verify_health_document_runtime
from .structured_indicator_parser import extract_structured_indicators
from .verification import mark_initial_verification

MAX_WORKER_INPUT_BYTES = 10 * 1024 * 1024


def _confidence_level(score: float) -> OCRConfidence:
    if score >= 0.8:
        return "high"
    if score >= 0.5:
        return "medium"
    return "low"


def _min_confidence(a: OCRConfidence, b: OCRConfidence) -> OCRConfidence:
    order = {"high": 3, "medium": 2, "low": 1, "unknown": 0}
    return a if order[a] <= order[b] else b


def _build_legacy_tesseract_response(file_bytes: bytes, mime_type: str) -> OCRResponse:
    config = load_tesseract_baseline_config()
    verify_tesseract_baseline_runtime(config)
    raw_text, score = extract_text_tesseract(file_bytes, mime_type)
    ocr_confidence = _confidence_level(score)
    indicators = extract_indicators(raw_text)
    indicators = apply_indicator_admissibility_v1(indicators, ocr_confidence=ocr_confidence)
    parser_confidence = get_overall_confidence(indicators)
    source = config.source_contract.model_dump()
    provenance = LegacyTesseractMechanismProvenance(
        status="verified",
        configuration_id="hdex-config-14af808ef184bf8b",
        mechanism_revision="health-document-tesseract-baseline-v1",
        execution_topology_revision="per-document-subprocess-v1",
        engine="tesseract",
        engine_version="5.5.0",
        wrapper="pytesseract",
        wrapper_version="0.3.13",
        languages=list(config.languages),
        language_artifacts=[item.model_dump(mode="json") for item in config.language_artifacts],
        pdf_strategy_revision="raster-all-pages-300dpi-v1",
        pdf_raster_dpi=300,
        indicator_parser_revision="legacy-regex-v1",
        indicator_parser_sha256=str(source["indicator_extractor_sha256"]),
        admissibility_policy_revision="ocr-indicator-admissibility-v1",
        ocr_service_sha256=str(source["ocr_service_sha256"]),
        worker_sha256=worker_source_sha256(),
    )
    return OCRResponse(
        status="completed",
        result=OCRResult(
            raw_text=raw_text,
            indicators=indicators,
            confidence=_min_confidence(ocr_confidence, parser_confidence),
            mechanism_provenance=provenance,
        ),
    )


def build_response(file_bytes: bytes, mime_type: str, configuration_id: str) -> OCRResponse:
    if configuration_id == "hdex-config-14af808ef184bf8b":
        return _build_legacy_tesseract_response(file_bytes, mime_type)
    config = get_health_document_configuration(configuration_id)
    if worker_source_sha256() != config.worker_sha256:
        raise RuntimeError("health-document worker source identity mismatch")
    runtime = verify_health_document_runtime(config)
    if runtime["configuration_id"] != configuration_id:
        raise RuntimeError("health-document runtime configuration identity mismatch")

    extracted = extract_document(file_bytes, mime_type, config)
    extraction_confidence = _confidence_level(extracted.confidence)
    indicators = extract_structured_indicators(extracted.parser_blocks)
    if config.verification_revision is not None:
        indicators = mark_initial_verification(
            indicators,
            page_methods={page.page: page.method for page in extracted.pages},
            verification_revision=config.verification_revision,
        )
    indicators = apply_indicator_admissibility(
        indicators,
        ocr_confidence=extraction_confidence,
        policy_revision=config.admissibility_policy_revision,
    )
    parser_confidence = get_overall_confidence(indicators)
    final_confidence = _min_confidence(extraction_confidence, parser_confidence)
    provenance = HealthDocumentMechanismProvenance.model_validate(config.provenance())
    return OCRResponse(
        status="completed",
        result=OCRResult(
            raw_text=extracted.raw_text,
            indicators=indicators,
            confidence=final_confidence,
            mechanism_provenance=provenance,
            pages=extracted.pages,
            source_blocks=extracted.source_blocks,
        ),
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--configuration-id", required=True)
    parser.add_argument("--mime-type", required=True)
    args = parser.parse_args()
    payload = sys.stdin.buffer.read(MAX_WORKER_INPUT_BYTES + 1)
    if len(payload) > MAX_WORKER_INPUT_BYTES:
        raise SystemExit("health-document worker input exceeds 10 MiB")
    response = build_response(payload, args.mime_type, args.configuration_id)
    sys.stdout.write(response.model_dump_json())
    return 0


def worker_source_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


if __name__ == "__main__":
    raise SystemExit(main())
