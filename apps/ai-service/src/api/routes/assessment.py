"""Typed observation-only Assessment API routes."""

from __future__ import annotations

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, ConfigDict, Field

from ...models.assessment import AssessmentAgentOutput
from ...services.assessment_service import get_assessment_service

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/assessment", tags=["assessment"])


class AssessmentRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    profile: dict[str, Any] = Field(default_factory=dict)
    rag_context: str = ""
    images: list[str] = Field(default_factory=list)
    posture_analysis: dict[str, Any] = Field(default_factory=dict)


@router.post("/generate", response_model=AssessmentAgentOutput)
async def generate_assessment(request: AssessmentRequest) -> AssessmentAgentOutput:
    try:
        result = await get_assessment_service().generate_assessment(**request.model_dump())
        return AssessmentAgentOutput.model_validate(result)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        logger.exception("Assessment generation failed")
        raise HTTPException(status_code=500, detail="Assessment generation failed") from exc
