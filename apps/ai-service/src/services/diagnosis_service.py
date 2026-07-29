"""Diagnosis and treatment plan generation service."""

import json
from typing import Any

from pydantic import BaseModel, Field, ValidationError

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage
from ..prompts.diagnosis import (
    DIAGNOSIS_SYSTEM_PROMPT,
    TREATMENT_SYSTEM_PROMPT,
    get_diagnosis_prompt,
    get_treatment_prompt,
)
from ..runtime.governance import guard_structured_output
from .faithfulness_checker import get_faithfulness_checker
from .red_flag_detector import get_red_flag_detector


class DiagnosisItem(BaseModel):
    """One possible posture diagnosis returned by the LLM."""

    name: str = Field(min_length=1)
    confidence: str = Field(min_length=1)
    severity: str = Field(min_length=1)
    basis: str = Field(min_length=1)
    typical_symptoms: str = Field(min_length=1)
    differential: str | None = None


class DiagnosisResponse(BaseModel):
    """Validated diagnosis response."""

    diagnoses: list[DiagnosisItem] = Field(min_length=1)


class ExercisePlan(BaseModel):
    """One corrective exercise in a treatment plan."""

    name: str = Field(min_length=1)
    description: str = Field(min_length=1)
    sets: str = Field(min_length=1)
    reps: str = Field(min_length=1)
    notes: str = ""


class TreatmentPlan(BaseModel):
    """Validated treatment plan."""

    goal: str = Field(min_length=1)
    duration_weeks: int = Field(gt=0)
    correction_exercises: list[ExercisePlan] = Field(default_factory=list)
    daily_habits: list[str] = Field(default_factory=list)
    nutrition_advice: str | None = None
    expected_timeline: str = Field(min_length=1)
    warning_signs: list[str] = Field(default_factory=list)


class TreatmentResponse(BaseModel):
    """Validated treatment response."""

    treatment_plan: TreatmentPlan


class DiagnosisService:
    """Service for generating diagnosis and treatment plans."""

    def __init__(self) -> None:
        self._ai = AIService()

    async def generate_diagnosis(
        self,
        extracted_info: list[dict[str, Any]],
        profile: dict[str, Any],
        conversation_summary: str = "",
        rag_context: str = "",
        rag_results: list[dict[str, Any]] | None = None,
        use_case: str = "llm.json",
    ) -> dict[str, Any]:
        """
        Generate possible diagnoses based on extracted symptoms.

        Args:
            extracted_info: List of extracted symptom dicts.
            profile: User profile dict.
            conversation_summary: Summary of conversation.
            rag_context: RAG context string for prompt.
            rag_results: Raw RAG results for citation.
            use_case: AIService route use_case for model selection.

        Returns:
            Dict with 'diagnoses' list and optional 'red_flags' and 'citations'.
        """
        # Check for red flags
        detector = get_red_flag_detector()
        red_flag_result = detector.detect(extracted_info, conversation_summary)

        user_prompt = get_diagnosis_prompt(
            extracted_info, profile, conversation_summary, rag_context
        )
        messages = [
            ChatMessage(role="system", content=DIAGNOSIS_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        response = await self._ai.generate(AiRequest(
            use_case=use_case,
            messages=messages,
            response_format="json_object",
            temperature=0.3,
            max_tokens=2048,
        ))

        try:
            raw_result = json.loads(response.text)
        except json.JSONDecodeError as e:
            raise ValueError(f"LLM returned invalid JSON for diagnosis: {e}") from e
        validated = self._validate(
            DiagnosisResponse, raw_result, "diagnosis"
        ).model_dump(exclude_none=True)

        # Add red flags if detected
        if red_flag_result.has_red_flags:
            validated["red_flags"] = red_flag_result.to_dict()

        # Add citations from RAG results
        if rag_results:
            validated["citations"] = rag_results

        # Forced governance gate before emit/persist (P2 Phase A).
        guarded = guard_structured_output(
            "diagnosis",
            validated,
            rag_results=rag_results,
            extracted_info=extracted_info,
        )
        return guarded.to_emit_dict()

    async def generate_treatment(
        self,
        confirmed_diagnosis: dict[str, Any],
        extracted_info: list[dict[str, Any]],
        profile: dict[str, Any],
        rag_context: str = "",
        rag_results: list[dict[str, Any]] | None = None,
        use_case: str = "llm.json",
    ) -> dict[str, Any]:
        """
        Generate a treatment plan based on confirmed diagnosis.

        Args:
            confirmed_diagnosis: The confirmed diagnosis dict.
            extracted_info: List of extracted symptom dicts.
            profile: User profile dict.
            rag_context: RAG context string for prompt.
            rag_results: Raw RAG results for faithfulness check and citation.
            use_case: AIService route use_case for model selection.

        Returns:
            Dict with 'treatment_plan', optional 'faithfulness' and 'citations'.
        """
        user_prompt = get_treatment_prompt(
            confirmed_diagnosis, extracted_info, profile, rag_context
        )
        messages = [
            ChatMessage(role="system", content=TREATMENT_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        response = await self._ai.generate(AiRequest(
            use_case=use_case,
            messages=messages,
            response_format="json_object",
            temperature=0.3,
            max_tokens=2048,
        ))

        try:
            raw_result = json.loads(response.text)
        except json.JSONDecodeError as e:
            raise ValueError(f"LLM returned invalid JSON for treatment: {e}") from e
        validated = self._validate(
            TreatmentResponse, raw_result, "treatment"
        ).model_dump(exclude_none=True)

        # Keep faithfulness annotation for clients that surface it, but the
        # forced gate below owns accept/degrade/reject decisions.
        if rag_results:
            checker = get_faithfulness_checker()
            faithfulness = checker.check_treatment_faithfulness(
                validated.get("treatment_plan", {}), rag_results
            )
            validated["faithfulness"] = faithfulness.to_dict()
            validated["citations"] = rag_results

        guarded = guard_structured_output(
            "treatment",
            validated,
            rag_results=rag_results,
            extracted_info=extracted_info,
        )
        return guarded.to_emit_dict()

    def _validate(
        self,
        schema: type[BaseModel],
        payload: dict[str, Any],
        label: str,
    ) -> BaseModel:
        """Validate parsed LLM JSON against the expected response schema."""
        try:
            return schema.model_validate(payload)
        except ValidationError as e:
            raise ValueError(f"Invalid {label} response schema: {e}") from e


# Singleton
_diagnosis_service: DiagnosisService | None = None


def get_diagnosis_service() -> DiagnosisService:
    """Get or create the default diagnosis service."""
    global _diagnosis_service
    if _diagnosis_service is None:
        _diagnosis_service = DiagnosisService()
    return _diagnosis_service
