"""OCR API routes for health report processing."""

import logging

from fastapi import APIRouter, File, HTTPException, UploadFile

from ...models.ocr import OCRConfidence, OCRResponse, OCRResult, TextExtractionResponse
from ...services.indicator_extractor import extract_indicators, get_overall_confidence
from ...services.ocr import extract_text
from ...services.report_indicator_admissibility import apply_indicator_admissibility

logger = logging.getLogger(__name__)

# 下面这一行使用 FastAPI 的 APIRouter 方法，传入前缀和 tags，
# 返回一个供后续接入路由的 router 对象。
router = APIRouter(prefix="/api/ocr", tags=["ocr"])

# 定义最大文件大小的常量 10MB
_MAX_FILE_SIZE = 10 * 1024 * 1024  # 10 MB


# 使用路由装饰符定义OCR提取端点,以及对应的响应模型 OCRResponse
@router.post("/extract", response_model=OCRResponse)
# 路由处理函数 extract_ocr，接收上传文件参数 file（类型 UploadFile），
# 使用 File(...) 表示为必填的文件上传参数。
async def extract_ocr(file: UploadFile = File(...)):
    """
    Extract text and health indicators from an uploaded file.

    Accepts image (JPEG, PNG, WebP) or PDF files.
    Returns structured OCR results with extracted health indicators.
    """
    # 校验文件类型：定义允许的类型集合，
    # 若上传文件类型不在集合中则用 HTTPException 抛出异常。
    allowed_types = {"image/jpeg", "image/png", "image/webp", "application/pdf"}
    if file.content_type not in allowed_types:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type: {file.content_type}. "
            f"Allowed: {', '.join(allowed_types)}",
        )

    # 处理文件上传与 OCR 提取逻辑；使用 try-except 捕获异常，
    # 出错时返回适当的 HTTP 响应。
    try:
        # Read file content
        file_bytes = await file.read()
        # 进行文件内容的读取，如果文件为空，抛出 HTTP 400 错误
        if not file_bytes:
            raise HTTPException(status_code=400, detail="Empty file")
        # 检查文件大小是否超过最大限制，如果超过，抛出 HTTP 413 错误
        if len(file_bytes) > _MAX_FILE_SIZE:
            raise HTTPException(
                status_code=413,
                detail=(
                    f"File too large ({len(file_bytes)} bytes). Maximum is {_MAX_FILE_SIZE} bytes."
                ),
            )

        # Extract text using OCR
        raw_text, confidence = extract_text(file_bytes, file.content_type)
        # 若提取文本为空，返回状态为 "completed" 的 OCRResponse，
        # 含空 raw_text、空 indicators 列表及低置信度。
        if not raw_text.strip():
            return OCRResponse(
                status="completed",
                result=OCRResult(
                    raw_text="",
                    indicators=[],
                    confidence="low",
                ),
            )

        # Extract health indicators. Extraction completion and evidence
        # admissibility are separate contracts: only high-confidence OCR +
        # high-confidence indicator parses are auto-admissible downstream.
        indicators = extract_indicators(raw_text)
        ocr_confidence = _confidence_level(confidence)
        indicators = apply_indicator_admissibility(
            indicators,
            ocr_confidence=ocr_confidence,
        )
        overall_confidence = get_overall_confidence(indicators)

        # Overall OCRResult confidence remains descriptive metadata. It is not
        # itself an evidence-admission decision.
        final_confidence = _min_confidence(
            ocr_confidence,
            overall_confidence,
        )
        # 返回状态为 "completed" 的 OCRResponse，
        # 含提取的 raw_text、indicators 列表与最终置信度。
        return OCRResponse(
            status="completed",
            result=OCRResult(
                raw_text=raw_text,
                indicators=indicators,
                confidence=final_confidence,
            ),
        )
    # 捕获 HTTPException 异常并重新抛出，以便 FastAPI 可以处理它们并返回适当的 HTTP 响应
    except HTTPException:
        raise
    # 捕获其他异常，记录异常信息，并抛出 HTTP 500 错误，表示服务器内部错误
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
                    f"File too large ({len(file_bytes)} bytes). Maximum is {_MAX_FILE_SIZE} bytes."
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


def _confidence_level(score: float) -> OCRConfidence:
    """Convert numeric confidence score to level string."""
    if score >= 0.8:
        return "high"
    elif score >= 0.5:
        return "medium"
    return "low"


def _min_confidence(a: OCRConfidence, b: OCRConfidence) -> OCRConfidence:
    """Return the lower of two confidence levels."""
    order = {"high": 3, "medium": 2, "low": 1}
    return a if order.get(a, 0) <= order.get(b, 0) else b
