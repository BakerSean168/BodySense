"""BodyState Diagnosis HTTP route."""

import logging
import os
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...services.diagnosis_service import get_diagnosis_service

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/api/diagnosis", tags=["diagnosis"])


class DiagnosisRequest(BaseModel):
    """One diagnosis run pinned to a durable BodyState revision."""

    user_id: str = ""
    body_state_revision: int = Field(gt=0)
    body_state: dict[str, Any]
    relevant_history: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    rag_context: str = ""
    rag_results: list[dict[str, Any]] | None = None


@router.post("/analyze")
async def analyze_diagnosis(request: DiagnosisRequest):
    if os.getenv("BODYSENSE_E2E_STUB_AI") == "1" and os.getenv(
        "ENVIRONMENT", "development"
    ).lower() in {"development", "test", "e2e"}:
        facts = request.body_state.get("facts") or []
        observations = request.body_state.get("observations") or []
        fact_ids = [
            str(item.get("id")) for item in facts if isinstance(item, dict) and item.get("id")
        ]
        observation_ids = [
            str(item.get("id"))
            for item in observations
            if isinstance(item, dict) and item.get("id")
        ]
        primary_fact = next((item for item in facts if isinstance(item, dict)), {})
        primary_observation = next(
            (item for item in observations if isinstance(item, dict)), {}
        )
        concern_key = str(
            primary_fact.get("concern_key")
            or primary_observation.get("concern_key")
            or "general"
        )
        body_region = str(
            primary_fact.get("body_region")
            or primary_observation.get("body_region")
            or "当前关注区域"
        )
        return {
            "status": "completed",
            "scope": "full_body",
            "summary": "E2E deterministic analysis",
            "candidates": [
                {
                    "concern_key": concern_key,
                    "name": f"E2E {body_region} load pattern",
                    "confidence": "中",
                    "severity": "轻度",
                    "evidence_strength": "中",
                    "impact": "久坐后不适",
                    "basis": f"由当前 BodyState 中的{body_region}事实支持",
                    "typical_symptoms": f"{body_region}不适",
                    "basis_fact_ids": fact_ids[:1],
                    "basis_observation_ids": observation_ids[:1],
                    "supporting_evidence_ids": [],
                    "counterevidence_ids": [],
                    "reasoning_summary": "deterministic e2e candidate",
                    "missing_information": [],
                    "safety_notes": [],
                }
            ],
            "cross_concern_patterns": [],
            "information_gaps": [],
            "safety_summary": {},
            "citations": [],
            "governance": {
                "kind": "diagnosis",
                "verdict": "accepted",
                "reasons": [],
                "issues": [],
            },
        }

    """Generate a typed possible-diagnosis analysis from durable BodyState."""

    try:
        return await get_diagnosis_service().generate_diagnosis(
            user_id=request.user_id,
            body_state_revision=request.body_state_revision,
            body_state=request.body_state,
            relevant_history=request.relevant_history,
            profile=request.profile,
            rag_context=request.rag_context,
            rag_results=request.rag_results,
        )
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except Exception as exc:
        logger.exception("Diagnosis generation failed")
        raise HTTPException(
            status_code=500,
            detail="Diagnosis generation failed. Please try again.",
        ) from exc
