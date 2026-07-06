"""Reassessment API routes."""

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...services.reassessment_service import get_reassessment_service

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/reassessment", tags=["reassessment"])


class ReassessmentRequest(BaseModel):
    """Request body for reassessment."""

    feedback: dict[str, Any] = Field(...)
    training_logs: list[dict[str, Any]] = Field(default_factory=list)
    current_plan: dict[str, Any] = Field(default_factory=dict)


@router.post("/analyze")
async def analyze_reassessment(request: ReassessmentRequest):
    """Analyze training feedback and generate adjustment suggestions."""
    try:
        service = get_reassessment_service()
        result = await service.analyze_feedback(
            feedback=request.feedback,
            training_logs=request.training_logs,
            current_plan=request.current_plan,
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception:
        logger.exception("Reassessment failed")
        raise HTTPException(
            status_code=500,
            detail="Reassessment failed. Please try again.",
        )
