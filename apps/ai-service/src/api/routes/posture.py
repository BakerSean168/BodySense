"""Posture photo analysis API route."""

import logging

from fastapi import APIRouter, File, Form, HTTPException, UploadFile

from ...models.posture import PostureAnalysis, PostureAnalysisResponse
from ...services.posture_analyzer import analyze_posture

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/posture", tags=["posture"])

_MAX_FILE_SIZE = 10 * 1024 * 1024  # 10 MB
_ALLOWED_TYPES = {"image/jpeg", "image/png", "image/webp"}
_VALID_VIEWS = {"front", "side", "back"}


@router.post("/analyze", response_model=PostureAnalysisResponse)
async def analyze(
    view: str = Form(...),
    file: UploadFile = File(...),
    configuration_id: str | None = Form(None),
):
    """Analyze a single-view posture photo and return a structured, governed result."""
    if view not in _VALID_VIEWS:
        raise HTTPException(status_code=400, detail=f"invalid view: {view}")

    if file.content_type not in _ALLOWED_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"Unsupported file type: {file.content_type}. "
            f"Allowed: {', '.join(sorted(_ALLOWED_TYPES))}",
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
        result = await analyze_posture(
            file_bytes,
            file.content_type,
            view,
            configuration_id=configuration_id,
        )
    except HTTPException:
        raise
    except Exception as e:
        logger.exception("posture analysis failed")
        raise HTTPException(status_code=502, detail=f"posture analysis failed: {e}") from e

    return PostureAnalysisResponse(status="completed", result=PostureAnalysis(**result))
