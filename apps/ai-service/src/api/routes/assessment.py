"""Typed observation-only Assessment API routes."""

from __future__ import annotations

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, ConfigDict, Field

from ...services.assessment_service import AssessmentService, get_assessment_service
from ...settings import e2e_stub_ai_enabled
from ...testing_support.deterministic_ai import deterministic_assessment_model

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/assessment", tags=["assessment"])


class AssessmentRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    profile: dict[str, Any] = Field(default_factory=dict)
    body_state: dict[str, Any] = Field(default_factory=dict)
    report_indicators: list[Any] = Field(default_factory=list)
    reviewed_report_evidence: list[Any] = Field(default_factory=list)
    rag_context: str = ""
    images: list[str] = Field(default_factory=list)
    posture_analysis: dict[str, Any] = Field(default_factory=dict)
    configuration_id: str = Field(min_length=1)


@router.post("/generate")
async def generate_assessment(request: AssessmentRequest):
    try:
        if e2e_stub_ai_enabled():
            # E2E remains deterministic but still executes the same evidence
            # catalog, typed contract and governance path as production.
            service = AssessmentService(
                model_resolver=lambda _config: deterministic_assessment_model(request.body_state)
            )
        else:
            service = get_assessment_service()
        return await service.generate_assessment(**request.model_dump())
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        logger.exception("Assessment generation failed")
        raise HTTPException(status_code=500, detail="Assessment generation failed") from exc
