"""Assessment API routes."""

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...services.assessment_service import get_assessment_service

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/assessment", tags=["assessment"])


class AssessmentRequest(BaseModel):
    """Request body for assessment generation."""

    profile: dict[str, Any] = Field(..., description="User profile data")
    rag_context: str = Field(default="", description="RAG context from knowledge base")
    images: list[str] | None = Field(default=None, description="Base64 encoded posture images")


class AssessmentResponse(BaseModel):
    """Response body for assessment generation."""

    health_grade: str
    dimension_scores: dict[str, Any]
    identified_issues: list[dict[str, Any]]
    improvement_summary: dict[str, Any]


@router.post("/generate", response_model=AssessmentResponse)
async def generate_assessment(request: AssessmentRequest):
    """Generate a health assessment report based on user profile."""
    try:
        service = get_assessment_service()
        result = await service.generate_assessment(
            profile=request.profile,
            rag_context=request.rag_context,
            images=request.images,
        )
        return AssessmentResponse(**result)
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception:
        logger.exception("Assessment generation failed")
        raise HTTPException(
            status_code=500,
            detail="Assessment generation failed. Please try again.",
        )
