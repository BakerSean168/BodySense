"""Diagnosis and treatment plan generation service."""

import json
from typing import Any

from ..prompts.diagnosis import (
    DIAGNOSIS_SYSTEM_PROMPT,
    TREATMENT_SYSTEM_PROMPT,
    get_diagnosis_prompt,
    get_treatment_prompt,
)
from .llm_provider import ChatMessage, get_llm_provider


class DiagnosisService:
    """Service for generating diagnosis and treatment plans."""

    async def generate_diagnosis(
        self,
        extracted_info: list[dict[str, Any]],
        profile: dict[str, Any],
        conversation_summary: str = "",
        rag_context: str = "",
    ) -> dict[str, Any]:
        """
        Generate possible diagnoses based on extracted symptoms.

        Returns:
            Dict with 'diagnoses' list.
        """
        provider = get_llm_provider()

        user_prompt = get_diagnosis_prompt(
            extracted_info, profile, conversation_summary, rag_context
        )
        messages = [
            ChatMessage(role="system", content=DIAGNOSIS_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        response = await provider.chat(
            messages=messages,
            temperature=0.3,
            max_tokens=2048,
        )

        return self._parse_json(response.content or "")

    async def generate_treatment(
        self,
        confirmed_diagnosis: dict[str, Any],
        extracted_info: list[dict[str, Any]],
        profile: dict[str, Any],
        rag_context: str = "",
    ) -> dict[str, Any]:
        """
        Generate a treatment plan based on confirmed diagnosis.

        Returns:
            Dict with 'treatment_plan'.
        """
        provider = get_llm_provider()

        user_prompt = get_treatment_prompt(
            confirmed_diagnosis, extracted_info, profile, rag_context
        )
        messages = [
            ChatMessage(role="system", content=TREATMENT_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        response = await provider.chat(
            messages=messages,
            temperature=0.3,
            max_tokens=2048,
        )

        return self._parse_json(response.content or "")

    def _parse_json(self, content: str) -> dict[str, Any]:
        """Parse JSON from LLM response."""
        try:
            return json.loads(content)
        except json.JSONDecodeError:
            pass

        if "```" in content:
            parts = content.split("```")
            for part in parts[1:]:
                if part.startswith("json"):
                    part = part[4:]
                part = part.strip()
                try:
                    return json.loads(part)
                except json.JSONDecodeError:
                    continue

        start = content.find("{")
        end = content.rfind("}") + 1
        if start >= 0 and end > start:
            try:
                return json.loads(content[start:end])
            except json.JSONDecodeError:
                pass

        raise ValueError("Could not parse JSON from LLM response")


# Singleton
_diagnosis_service: DiagnosisService | None = None


def get_diagnosis_service() -> DiagnosisService:
    """Get or create the default diagnosis service."""
    global _diagnosis_service
    if _diagnosis_service is None:
        _diagnosis_service = DiagnosisService()
    return _diagnosis_service
