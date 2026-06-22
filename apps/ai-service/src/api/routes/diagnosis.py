"""Diagnosis and treatment API routes."""

from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...services.diagnosis_service import get_diagnosis_service

router = APIRouter(prefix="/api/diagnosis", tags=["diagnosis"])


class DiagnosisRequest(BaseModel):
    """Request body for diagnosis generation."""

    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    conversation_summary: str = Field(default="")
    rag_context: str = Field(default="")


class TreatmentRequest(BaseModel):
    """Request body for treatment plan generation."""

    confirmed_diagnosis: dict[str, Any] = Field(...)
    extracted_info: list[dict[str, Any]] = Field(default_factory=list)
    profile: dict[str, Any] = Field(default_factory=dict)
    rag_context: str = Field(default="")


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
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Diagnosis failed: {e!s}")


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
        )
        return result
    except ValueError as e:
        raise HTTPException(status_code=422, detail=str(e))
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Treatment generation failed: {e!s}")
