"""OCR API routes for health report processing."""

import logging

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from ...document_pipeline.subprocess_runner import run_health_document_worker
from ...models.ocr import OCRResponse, TextExtractionResponse

logger = logging.getLogger(__name__)

# 下面这一行使用 FastAPI 的 APIRouter 方法，传入前缀和 tags，
# 返回一个供后续接入路由的 router 对象。
router = APIRouter(prefix="/api/ocr", tags=["ocr"])

# 定义最大文件大小的常量 10MB
_MAX_FILE_SIZE = 10 * 1024 * 1024  # 10 MB


# 使用路由装饰符定义OCR提取端点,以及对应的响应模型 OCRResponse
@router.post("/extract", response_model=OCRResponse)
async def extract_ocr(
    file: UploadFile = File(...),
    configuration_id: str = Form(...),
) -> OCRResponse:
    """Extract source-grounded health-document evidence through one bounded worker."""
    allowed_types = {"image/jpeg", "image/png", "image/webp", "application/pdf"}
    if file.content_type not in allowed_types:
        raise HTTPException(
            status_code=400,
            detail=(
                f"Unsupported file type: {file.content_type}. Allowed: {', '.join(allowed_types)}"
            ),
        )
    file_bytes = await file.read()
    if not file_bytes:
        raise HTTPException(status_code=400, detail="Empty file")
    if len(file_bytes) > _MAX_FILE_SIZE:
        raise HTTPException(
            status_code=413,
            detail=f"File too large ({len(file_bytes)} bytes). Maximum is {_MAX_FILE_SIZE} bytes.",
        )
    try:
        return await run_health_document_worker(
            file_bytes,
            file.content_type or "",
            configuration_id,
        )
    except ValueError as exc:
        raise HTTPException(
            status_code=400, detail="Unknown health-document configuration"
        ) from exc
    except Exception as exc:
        logger.exception("Health-document extraction worker failed")
        raise HTTPException(status_code=500, detail="Health-document extraction failed") from exc


@router.post("/extract-text", response_model=TextExtractionResponse)
async def extract_text_only(
    file: UploadFile = File(...),
    configuration_id: str = Form(...),
) -> TextExtractionResponse:
    """Return raw extracted text using the same governed document mechanism."""
    allowed_types = {"image/jpeg", "image/png", "image/webp", "application/pdf"}
    if file.content_type not in allowed_types:
        raise HTTPException(status_code=400, detail="Unsupported file type")
    file_bytes = await file.read()
    if not file_bytes:
        raise HTTPException(status_code=400, detail="Empty file")
    if len(file_bytes) > _MAX_FILE_SIZE:
        raise HTTPException(status_code=413, detail="File too large")
    try:
        response = await run_health_document_worker(
            file_bytes,
            file.content_type or "",
            configuration_id,
        )
    except ValueError as exc:
        raise HTTPException(
            status_code=400, detail="Unknown health-document configuration"
        ) from exc
    except Exception as exc:
        logger.exception("Health-document text extraction worker failed")
        raise HTTPException(status_code=500, detail="Health-document extraction failed") from exc
    return TextExtractionResponse(
        text=response.result.raw_text, pages=max(1, len(response.result.pages))
    )
