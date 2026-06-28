"""Diagnosis and treatment API routes."""

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...services.diagnosis_service import get_diagnosis_service

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/diagnosis", tags=["diagnosis"])


class DiagnosisRequest(BaseModel):
    """Request body for diagnosis generation."""

    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    conversation_summary: str = Field(default="")
    rag_context: str = Field(default="")
    rag_results: list[dict[str, Any]] | None = Field(default=None)
    use_case: str = Field(default="llm.json")


class TreatmentRequest(BaseModel):
    """Request body for treatment plan generation."""

    confirmed_diagnosis: dict[str, Any] = Field(...)
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    rag_context: str = Field(default="")
    rag_results: list[dict[str, Any]] | None = Field(default=None)
    use_case: str = Field(default="llm.json")


@router.post("/analyze")
async def analyze_diagnosis(request: DiagnosisRequest):
    """Generate possible diagnoses based on extracted symptoms."""
    try:
        service = get_diagnosis_service()
        result = await service.generate_diagnosis(
            extracted_info=request.extracted_info,
            profile=request.profile,
            conversation_summary=request.conversation_summary,
            rag_context=request.rag_context,
            rag_results=request.rag_results,
            use_case=request.use_case,
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception:
        logger.exception("Diagnosis generation failed")
        raise HTTPException(
            status_code=500,
            detail="Diagnosis generation failed. Please try again.",
        )


@router.post("/treatment")
async def generate_treatment(request: TreatmentRequest):
    """Generate a treatment plan based on confirmed diagnosis."""
    try:
        service = get_diagnosis_service()
        result = await service.generate_treatment(
            confirmed_diagnosis=request.confirmed_diagnosis,
            extracted_info=request.extracted_info,
            profile=request.profile,
            rag_context=request.rag_context,
            rag_results=request.rag_results,
            use_case=request.use_case,
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception:
        logger.exception("Treatment generation failed")
        raise HTTPException(
            status_code=500,
            detail="Treatment generation failed. Please try again.",
        )
