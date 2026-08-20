"""Typed Treatment proposal HTTP route."""

from __future__ import annotations

import logging
import os
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, ConfigDict, Field

from ...services.treatment_agent_service import get_treatment_agent_service

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/treatment", tags=["treatment"])


class TreatmentRecommendationRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    user_id: str = ""
    body_state_revision: int = Field(ge=0)
    body_state: dict[str, Any]
    diagnosis_analysis: dict[str, Any]
    candidate_assessments: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    user_constraints: dict[str, Any] = Field(default_factory=dict)
    evidence: list[dict[str, Any]] = Field(default_factory=list)


@router.post("/recommend")
async def recommend_treatment(request: TreatmentRecommendationRequest):
    if os.getenv("BODYSENSE_E2E_STUB_AI") == "1" and os.getenv(
        "ENVIRONMENT", "development"
    ).lower() in {"development", "test", "e2e"}:
        return {
            "status": "proposed",
            "summary": "E2E deterministic intervention proposal",
            "goal": "Reduce neck load",
            "duration_weeks": 4,
            "interventions": [
                {
                    "kind": "exercise",
                    "title": "Controlled chin tuck",
                    "description": "Low-load controlled movement",
                    "prescription": {"sets": "2", "reps": "8", "notes": "stop on worsening"},
                }
            ],
            "daily_habits": ["change position regularly"],
            "expected_timeline": "review after one week",
            "warning_signs": ["new numbness or weakness"],
            "review_triggers": ["symptoms worsen"],
            "safety_notes": ["do not continue through worsening symptoms"],
            "evidence_ids": [],
            "governance": {
                "kind": "treatment",
                "verdict": "accepted",
                "reasons": [],
                "issues": [],
            },
        }

    try:
        return await get_treatment_agent_service().recommend(**request.model_dump())
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        logger.exception("Treatment Agent recommendation failed")
        raise HTTPException(status_code=500, detail="Treatment recommendation failed") from exc
