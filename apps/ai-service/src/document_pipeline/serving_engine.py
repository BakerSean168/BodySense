"""Current health-document extraction mechanism.

This module is executed only inside the short-lived per-document worker. The
FastAPI parent never imports RapidOCR, so ONNX/native allocations die with the
worker instead of accumulating across document jobs.
"""

from __future__ import annotations

import gc
import hashlib
import os
from dataclasses import dataclass
from functools import lru_cache
from importlib import metadata
from pathlib import Path
from typing import Any

import fitz

from ..configuration.health_document_config import (
    HealthDocumentManifest,
    get_default_health_document_configuration,
    get_health_document_configuration,
)
from ..models.ocr import DocumentPageEvidence, DocumentSourceBlock
from ..services.report_indicator_admissibility import admissibility_policy_source_sha256
from .strategy import evaluate_native_text_quality, native_text_quality_policy_fingerprint
from .structured_indicator_parser import SourceTextBlock, parser_source_sha256
from .verification import verification_policy_sha256

SERVICE_ROOT = Path(__file__).resolve().parents[2]
MODEL_ROOT_ENV = "BODYSENSE_HEALTH_DOCUMENT_MODEL_ROOT"
DEFAULT_MODEL_ROOT = SERVICE_ROOT / "models" / "health-document" / "ppocrv6-small-v1"


class HealthDocumentMechanismUnavailableError(RuntimeError):
    pass


class HealthDocumentMechanismIdentityError(RuntimeError):
    pass


@dataclass(frozen=True)
class ExtractedDocument:
    raw_text: str
    confidence: float
    parser_blocks: list[SourceTextBlock]
    pages: list[DocumentPageEvidence]
    source_blocks: list[DocumentSourceBlock]


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _assert_equal(label: str, actual: str, expected: str) -> None:
    if actual != expected:
        raise HealthDocumentMechanismIdentityError(
            f"{label} mismatch: got {actual!r} want {expected!r}"
        )


def model_root() -> Path:
    return Path(os.getenv(MODEL_ROOT_ENV, str(DEFAULT_MODEL_ROOT))).expanduser().resolve()


def verify_health_document_runtime(
    config: HealthDocumentManifest | None = None,
) -> dict[str, object]:
    config = config or get_default_health_document_configuration()
    _assert_equal("RapidOCR version", metadata.version("rapidocr"), config.ocr_engine_version)
    _assert_equal("ONNXRuntime version", metadata.version("onnxruntime"), config.runtime_version)
    _assert_equal("PyMuPDF version", metadata.version("PyMuPDF"), config.native_text_engine_version)
    _assert_equal(
        "native text quality policy revision",
        config.native_text_quality_policy_revision,
        "health-document-native-text-quality-v1",
    )
    _assert_equal(
        "native text quality policy hash",
        native_text_quality_policy_fingerprint(),
        config.native_text_quality_policy_sha256,
    )
    _assert_equal("indicator parser hash", parser_source_sha256(), config.indicator_parser_sha256)
    _assert_equal("serving engine hash", sha256_file(Path(__file__)), config.engine_adapter_sha256)
    if config.admissibility_policy_sha256 is not None:
        _assert_equal(
            "admissibility policy hash",
            admissibility_policy_source_sha256(),
            config.admissibility_policy_sha256,
        )
    if config.verification_policy_sha256 is not None:
        _assert_equal(
            "verification policy hash",
            verification_policy_sha256(),
            config.verification_policy_sha256,
        )

    root = model_root()
    verified_models: list[dict[str, str]] = []
    for artifact in config.model_artifacts:
        path = root / artifact.filename
        if not path.is_file():
            raise HealthDocumentMechanismUnavailableError(f"model artifact missing: {path}")
        _assert_equal(f"{artifact.role} model sha256", sha256_file(path), artifact.sha256)
        verified_models.append(artifact.model_dump(mode="json"))
    return {
        "configuration_id": config.configuration_id,
        "fingerprint": config.fingerprint,
        "models": verified_models,
    }


@lru_cache(maxsize=8)
def _rapidocr_engine(configuration_id: str) -> Any:
    config = get_health_document_configuration(configuration_id)
    verify_health_document_runtime(config)
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
        raise HealthDocumentMechanismUnavailableError("RapidOCR package unavailable") from exc

    by_role = {item.role: item for item in config.model_artifacts}
    root = model_root()
    return RapidOCR(
        params={
            "Det.engine_type": EngineType.ONNXRUNTIME,
            "Det.lang_type": LangDet.CH,
            "Det.model_type": ModelType.SMALL,
            "Det.ocr_version": OCRVersion.PPOCRV6,
            "Det.model_path": str(root / by_role["det"].filename),
            "Global.max_side_len": config.global_max_side_len or 2000,
            "EngineConfig.onnxruntime.intra_op_num_threads": (
                config.ort_intra_op_num_threads
                if config.ort_intra_op_num_threads is not None
                else -1
            ),
            "EngineConfig.onnxruntime.inter_op_num_threads": (
                config.ort_inter_op_num_threads
                if config.ort_inter_op_num_threads is not None
                else -1
            ),
            "Det.limit_type": config.detector_limit_type,
            "Det.limit_side_len": config.detector_limit_side_len,
            "Rec.engine_type": EngineType.ONNXRUNTIME,
            "Rec.lang_type": LangRec.CH,
            "Rec.model_type": ModelType.SMALL,
            "Rec.ocr_version": OCRVersion.PPOCRV6,
            "Rec.model_path": str(root / by_role["rec"].filename),
            "Rec.rec_batch_num": config.rec_batch_num or 6,
            "Cls.engine_type": EngineType.ONNXRUNTIME,
            "Cls.lang_type": LangDet.CH,
            "Cls.model_type": ModelType.MOBILE,
            "Cls.ocr_version": OCRVersion.PPOCRV4,
            "Cls.model_path": str(root / by_role["cls"].filename),
            "Cls.cls_batch_num": config.cls_batch_num or 6,
            "Global.log_level": "warning",
        }
    )


def _hash_text(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()


@dataclass(frozen=True)
class _OCRCell:
    source_ref: str
    text: str
    bbox: tuple[float, ...] | None

    @property
    def x_min(self) -> float:
        if not self.bbox:
            return 0.0
        return min(self.bbox[0::2])

    @property
    def y_min(self) -> float:
        if not self.bbox:
            return 0.0
        return min(self.bbox[1::2])

    @property
    def y_max(self) -> float:
        if not self.bbox:
            return 0.0
        return max(self.bbox[1::2])

    @property
    def y_center(self) -> float:
        return (self.y_min + self.y_max) / 2.0

    @property
    def height(self) -> float:
        return max(1.0, self.y_max - self.y_min)


def _reconstruct_ocr_rows(cells: list[_OCRCell], *, page: int) -> list[SourceTextBlock]:
    positioned = [cell for cell in cells if cell.bbox]
    unpositioned = [cell for cell in cells if not cell.bbox]
    if not positioned:
        return [
            SourceTextBlock(
                source_ref=cell.source_ref,
                source_refs=(cell.source_ref,),
                page=page,
                text=cell.text,
            )
            for cell in cells
        ]

    heights = sorted(cell.height for cell in positioned)
    median_height = heights[len(heights) // 2]
    tolerance = max(6.0, median_height * 0.45)
    rows: list[list[_OCRCell]] = []
    row_centers: list[float] = []
    for cell in sorted(positioned, key=lambda item: (item.y_center, item.x_min)):
        if rows and abs(cell.y_center - row_centers[-1]) <= tolerance:
            rows[-1].append(cell)
            row_centers[-1] = sum(item.y_center for item in rows[-1]) / len(rows[-1])
        else:
            rows.append([cell])
            row_centers.append(cell.y_center)

    parser_blocks: list[SourceTextBlock] = []
    for row in rows:
        ordered = sorted(row, key=lambda item: item.x_min)
        refs = tuple(item.source_ref for item in ordered)
        parser_blocks.append(
            SourceTextBlock(
                source_ref=refs[0],
                source_refs=refs,
                page=page,
                text=" ".join(item.text for item in ordered),
            )
        )
    parser_blocks.extend(
        SourceTextBlock(
            source_ref=cell.source_ref,
            source_refs=(cell.source_ref,),
            page=page,
            text=cell.text,
        )
        for cell in unpositioned
    )
    return parser_blocks


def _rapidocr_blocks(
    image_bytes: bytes, *, page: int, configuration_id: str
) -> tuple[list[SourceTextBlock], list[DocumentSourceBlock], float]:
    result = _rapidocr_engine(configuration_id)(image_bytes)
    texts = list(result.txts or ())
    scores = [float(value) for value in (result.scores or ())]
    boxes = list(result.boxes) if result.boxes is not None else []
    cells: list[_OCRCell] = []
    source_blocks: list[DocumentSourceBlock] = []
    for index, text in enumerate(texts, start=1):
        normalized = str(text).strip()
        if not normalized:
            continue
        source_ref = f"page:{page}:ocr-block:{index}"
        score = scores[index - 1] if index - 1 < len(scores) else None
        bbox: list[float] | None = None
        if index - 1 < len(boxes):
            points = boxes[index - 1]
            bbox = [
                float(value)
                for point in list(points)  # pyright: ignore[reportGeneralTypeIssues]
                for value in list(point)  # pyright: ignore[reportGeneralTypeIssues]
            ]
        cells.append(
            _OCRCell(
                source_ref=source_ref,
                text=normalized,
                bbox=tuple(bbox) if bbox is not None else None,
            )
        )
        source_blocks.append(
            DocumentSourceBlock(
                source_ref=source_ref,
                page=page,
                method="rapidocr",
                bbox=bbox,
                coordinate_space="ocr_pixels",
                confidence=score,
                text_sha256=_hash_text(normalized),
            )
        )
    parser_blocks = _reconstruct_ocr_rows(cells, page=page)
    confidence = sum(scores) / len(scores) if scores else 0.0
    return parser_blocks, source_blocks, confidence


def _native_blocks(
    page_obj: fitz.Page, *, page: int
) -> tuple[list[SourceTextBlock], list[DocumentSourceBlock], str]:
    raw_blocks = page_obj.get_text("blocks", sort=True)
    parser_blocks: list[SourceTextBlock] = []
    source_blocks: list[DocumentSourceBlock] = []
    text_parts: list[str] = []
    for index, block in enumerate(raw_blocks, start=1):
        text = str(block[4]).strip()
        if not text:
            continue
        source_ref = f"page:{page}:native-block:{index}"
        parser_blocks.append(SourceTextBlock(source_ref=source_ref, page=page, text=text))
        source_blocks.append(
            DocumentSourceBlock(
                source_ref=source_ref,
                page=page,
                method="native_pdf_text",
                bbox=[float(block[0]), float(block[1]), float(block[2]), float(block[3])],
                coordinate_space="pdf_points",
                confidence=1.0,
                text_sha256=_hash_text(text),
            )
        )
        text_parts.append(text)
    return parser_blocks, source_blocks, "\n".join(text_parts)


def extract_document(
    file_bytes: bytes,
    mime_type: str,
    config: HealthDocumentManifest | None = None,
) -> ExtractedDocument:
    config = config or get_default_health_document_configuration()
    verify_health_document_runtime(config)
    pages: list[DocumentPageEvidence] = []
    all_parser_blocks: list[SourceTextBlock] = []
    all_source_blocks: list[DocumentSourceBlock] = []
    raw_pages: list[str] = []
    confidences: list[float] = []

    if mime_type == "application/pdf":
        doc = fitz.open(stream=file_bytes, filetype="pdf")
        try:
            for page_index in range(len(doc)):
                page_number = page_index + 1
                page_obj = doc.load_page(page_index)
                native_parser, native_sources, native_text = _native_blocks(
                    page_obj, page=page_number
                )
                decision = evaluate_native_text_quality(native_text)
                if decision.usable:
                    parser_blocks = native_parser
                    source_blocks = native_sources
                    text = native_text
                    confidence = 1.0
                    method = "native_pdf_text"
                else:
                    pix = page_obj.get_pixmap(dpi=config.pdf_raster_dpi)
                    image_bytes = pix.tobytes("png")
                    parser_blocks, source_blocks, confidence = _rapidocr_blocks(
                        image_bytes, page=page_number, configuration_id=config.configuration_id
                    )
                    text = "\n".join(block.text for block in parser_blocks)
                    method = "rapidocr"
                    del image_bytes
                    del pix
                    gc.collect()
                all_parser_blocks.extend(parser_blocks)
                all_source_blocks.extend(source_blocks)
                raw_pages.append(
                    f"--- Page {page_number} ---\n{text}" if text else f"--- Page {page_number} ---"
                )
                confidences.append(confidence)
                pages.append(
                    DocumentPageEvidence(
                        page=page_number,
                        method=method,
                        source_refs=[block.source_ref for block in source_blocks],
                        confidence=confidence,
                        native_text_quality_policy_revision=decision.policy_revision,
                        native_text_quality_reason_codes=list(decision.reason_codes),
                    )
                )
        finally:
            doc.close()
    elif mime_type.startswith("image/"):
        parser_blocks, source_blocks, confidence = _rapidocr_blocks(
            file_bytes, page=1, configuration_id=config.configuration_id
        )
        all_parser_blocks.extend(parser_blocks)
        all_source_blocks.extend(source_blocks)
        text = "\n".join(block.text for block in parser_blocks)
        raw_pages.append(text)
        confidences.append(confidence)
        pages.append(
            DocumentPageEvidence(
                page=1,
                method="rapidocr",
                source_refs=[block.source_ref for block in source_blocks],
                confidence=confidence,
                native_text_quality_reason_codes=["not_applicable_image_input"],
            )
        )
    else:
        raise ValueError(f"unsupported health-document mime_type: {mime_type}")

    return ExtractedDocument(
        raw_text="\n\n".join(raw_pages),
        confidence=min(confidences) if confidences else 0.0,
        parser_blocks=all_parser_blocks,
        pages=pages,
        source_blocks=all_source_blocks,
    )
