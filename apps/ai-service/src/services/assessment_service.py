"""Assessment service for generating health assessment reports."""

import json
from typing import Any

from ..prompts.assessment import ASSESSMENT_SYSTEM_PROMPT, get_assessment_prompt
from .llm_provider import ChatMessage, get_llm_provider


class AssessmentService:
    """Service for generating health assessment reports."""

    async def generate_assessment(
        self,
        profile: dict[str, Any],
        rag_context: str = "",
    ) -> dict[str, Any]:
        """
        Generate a health assessment report based on user profile.

        Args:
            profile: User profile data.
            rag_context: Optional RAG context from knowledge base.

        Returns:
            Assessment result as a dict with health_grade, dimension_scores,
            identified_issues, and improvement_summary.
        """
        provider = get_llm_provider()

        # Build messages
        user_prompt = get_assessment_prompt(profile, rag_context)
        messages = [
            ChatMessage(role="system", content=ASSESSMENT_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        # Call LLM
        response = await provider.chat(
            messages=messages,
            temperature=0.3,  # Lower temperature for more consistent output
            max_tokens=2048,
        )

        # Parse JSON response
        content = response.content or ""

        # Try to extract JSON from the response
        result = self._parse_json_response(content)

        # Validate required fields
        self._validate_result(result)

        return result

    def _parse_json_response(self, content: str) -> dict[str, Any]:
        """Parse JSON from LLM response, handling markdown code blocks."""
        # Try direct JSON parse first
        try:
            return json.loads(content)
        except json.JSONDecodeError:
            pass

        # Try extracting from markdown code block
        if "```" in content:
            parts = content.split("```")
            for part in parts[1:]:
                # Remove language identifier
                if part.startswith("json"):
                    part = part[4:]
                part = part.strip()
                try:
                    return json.loads(part)
                except json.JSONDecodeError:
                    continue

        # Try finding JSON object in the text
        start = content.find("{")
        end = content.rfind("}") + 1
        if start >= 0 and end > start:
            try:
                return json.loads(content[start:end])
            except json.JSONDecodeError:
                pass

        raise ValueError("Could not parse assessment JSON from LLM response")

    def _validate_result(self, result: dict[str, Any]) -> None:
        """Validate the assessment result has required fields."""
        required = ["health_grade", "dimension_scores", "identified_issues", "improvement_summary"]
        for field in required:
            if field not in result:
                raise ValueError(f"Missing required field: {field}")

        # Validate health grade
        valid_grades = {"A", "B", "C", "D"}
        if result["health_grade"] not in valid_grades:
            raise ValueError(f"Invalid health grade: {result['health_grade']}")

        # Validate dimension scores
        scores = result["dimension_scores"]
        for key in ["posture", "exercise", "lifestyle", "injury_risk", "overall"]:
            if key not in scores:
                raise ValueError(f"Missing dimension score: {key}")
            if not isinstance(scores[key], (int, float)):
                raise ValueError(f"Invalid score type for {key}")


# Singleton instance
_assessment_service: AssessmentService | None = None


def get_assessment_service() -> AssessmentService:
    """Get or create the default assessment service."""
    global _assessment_service
    if _assessment_service is None:
        _assessment_service = AssessmentService()
    return _assessment_service
