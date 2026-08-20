"""Typed observation-only Assessment API routes."""

from __future__ import annotations

import logging
import os
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, ConfigDict, Field

from ...services.assessment_service import get_assessment_service

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/assessment", tags=["assessment"])


class AssessmentRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    profile: dict[str, Any] = Field(default_factory=dict)
    rag_context: str = ""
    images: list[str] = Field(default_factory=list)
    posture_analysis: dict[str, Any] = Field(default_factory=dict)
    configuration_id: str | None = None


@router.post("/generate")
async def generate_assessment(request: AssessmentRequest):
    if os.getenv("BODYSENSE_E2E_STUB_AI") == "1" and os.getenv(
        "ENVIRONMENT", "development"
    ).lower() in {"development", "test", "e2e"}:
        return {
            "status": "completed",
            "health_grade": "B",
            "dimension_scores": {
                "posture": 62.0,
                "exercise": 70.0,
                "lifestyle": 75.0,
                "injury_risk": 30.0,
                "overall": 68.0,
            },
            "observations": [
                {
                    "kind": "posture",
                    "body_region": "neck",
                    "label": "Forward head tendency",
                    "description": "Observed forward head posture indicator",
                    "severity": "轻度",
                    "confidence": "中",
                    "method": "assessment",
                    "condition": {},
                }
            ],
            "summary": "E2E deterministic assessment report",
            "information_gaps": [],
            "safety_notes": [],
            "agent_configuration": (
                {}
                if request.configuration_id is None
                else {"id": request.configuration_id, "role": "assessment"}
            ),
            "execution_provenance": {
                "status": "executed",
                "runtime": "pydantic-ai",
                "logical_model": "bodysense-structured",
            },
        }

    try:
        return await get_assessment_service().generate_assessment(
            **request.model_dump()
        )
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        logger.exception("Assessment generation failed")
        raise HTTPException(status_code=500, detail="Assessment generation failed") from exc
