"""OCR API routes for health report processing."""

import logging

from fastapi import APIRouter, File, HTTPException, UploadFile

from ...models.ocr import OCRResponse, OCRResult, TextExtractionResponse
from ...services.indicator_extractor import extract_indicators, get_overall_confidence
from ...services.ocr import extract_text

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/ocr", tags=["ocr"])

_MAX_FILE_SIZE = 10 * 1024 * 1024  # 10 MB


@router.post("/extract", response_model=OCRResponse)
async def extract_ocr(file: UploadFile = File(...)):
    """
    Extract text and health indicators from an uploaded file.

    Accepts image (JPEG, PNG, WebP) or PDF files.
    Returns structured OCR results with extracted health indicators.
    """
    # Validate file type
    allowed_types = {"image/jpeg", "image/png", "image/webp", "application/pdf"}
    if file.content_type not in allowed_types:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type: {file.content_type}. "
            f"Allowed: {', '.join(allowed_types)}",
        )

    try:
        # Read file content
        file_bytes = await file.read()

        if not file_bytes:
            raise HTTPException(status_code=400, detail="Empty file")

        if len(file_bytes) > _MAX_FILE_SIZE:
            raise HTTPException(
                status_code=413,
                detail=(
                    f"File too large ({len(file_bytes)} bytes). "
                    f"Maximum is {_MAX_FILE_SIZE} bytes."
                ),
            )

        # Extract text using OCR
        raw_text, confidence = extract_text(file_bytes, file.content_type)

        if not raw_text.strip():
            return OCRResponse(
                status="completed",
                result=OCRResult(
                    raw_text="",
                    indicators=[],
                    confidence="low",
                ),
            )

        # Extract health indicators
        indicators = extract_indicators(raw_text)
        overall_confidence = get_overall_confidence(indicators)

        # Use the lower of OCR confidence and indicator confidence
        final_confidence = _min_confidence(
            _confidence_level(confidence),
            overall_confidence,
        )

        return OCRResponse(
            status="completed",
            result=OCRResult(
                raw_text=raw_text,
                indicators=indicators,
                confidence=final_confidence,
            ),
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("OCR extraction failed")
        raise HTTPException(
            status_code=500,
            detail=f"OCR processing failed: {str(e)}",
        )


@router.post("/extract-text", response_model=TextExtractionResponse)
async def extract_text_only(file: UploadFile = File(...)):
    """
    Extract raw text from an uploaded file without indicator parsing.

    Accepts image (JPEG, PNG, WebP) or PDF files.
    Returns the raw extracted text.
    """
    # Validate file type
    allowed_types = {"image/jpeg", "image/png", "image/webp", "application/pdf"}
    if file.content_type not in allowed_types:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type: {file.content_type}. "
            f"Allowed: {', '.join(allowed_types)}",
        )

    try:
        file_bytes = await file.read()

        if not file_bytes:
            raise HTTPException(status_code=400, detail="Empty file")

        if len(file_bytes) > _MAX_FILE_SIZE:
            raise HTTPException(
                status_code=413,
                detail=(
                    f"File too large ({len(file_bytes)} bytes). "
                    f"Maximum is {_MAX_FILE_SIZE} bytes."
                ),
            )

        raw_text, _ = extract_text(file_bytes, file.content_type)

        # Count pages (approximate for PDFs)
        pages = raw_text.count("--- Page ") + 1 if "--- Page " in raw_text else 1

        return TextExtractionResponse(
            text=raw_text,
            pages=pages,
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Text extraction failed")
        raise HTTPException(
            status_code=500,
            detail=f"Text extraction failed: {str(e)}",
        )


def _confidence_level(score: float) -> str:
    """Convert numeric confidence score to level string."""
    if score >= 0.8:
        return "high"
    elif score >= 0.5:
        return "medium"
    return "low"


def _min_confidence(a: str, b: str) -> str:
    """Return the lower of two confidence levels."""
    order = {"high": 3, "medium": 2, "low": 1}
    return a if order.get(a, 0) <= order.get(b, 0) else b
