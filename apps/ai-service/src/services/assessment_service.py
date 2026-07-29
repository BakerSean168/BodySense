"""Assessment service for generating health assessment reports."""

from typing import Any

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage
from ..prompts.assessment import ASSESSMENT_SYSTEM_PROMPT, get_assessment_prompt


class AssessmentService:
    """Service for generating health assessment reports."""

    def __init__(self) -> None:
        self._ai = AIService()

    async def generate_assessment(
        self,
        profile: dict[str, Any],
        rag_context: str = "",
        images: list[str] | None = None,
        posture_analysis: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """
        Generate a health assessment report based on user profile.

        Args:
            profile: User profile data.
            rag_context: Optional RAG context from knowledge base.
            images: Optional residual Base64 posture images for views that do
                not yet have a completed ``analysis_result``. Go only sends
                these for incomplete views; they must still be attached even
                when other views already have stored analysis.
            posture_analysis: Optional completed three-view analysis summary
                from ``user_uploads.analysis_result``. Embedded into the text
                prompt so completed views are not re-sent as raw pixels.

        Returns:
            Assessment result as a dict with health_grade, dimension_scores,
            identified_issues, and improvement_summary.
        """
        # Always embed stored analysis into the text prompt when present.
        user_prompt = get_assessment_prompt(
            profile,
            rag_context,
            posture_analysis=posture_analysis,
        )

        # Hybrid: residual images (incomplete views) ride alongside stored
        # analysis text. Only skip multimodal blocks when there are no images.
        residual = [img for img in (images or []) if img]
        if residual:
            content_list: list[dict[str, Any]] = [
                {"type": "text", "text": user_prompt}
            ]
            for img in residual:
                if not img.startswith("data:"):
                    img = f"data:image/jpeg;base64,{img}"
                content_list.append(
                    {
                        "type": "image_url",
                        "image_url": {"url": img},
                    }
                )
            user_msg = ChatMessage(role="user", content=content_list)
        else:
            user_msg = ChatMessage(role="user", content=user_prompt)

        messages = [
            ChatMessage(role="system", content=ASSESSMENT_SYSTEM_PROMPT),
            user_msg,
        ]

        # Call LLM via AIService (json_mode guarantees valid JSON)
        response = await self._ai.generate(AiRequest(
            use_case="llm.json",
            messages=messages,
            response_format="json_object",
            temperature=0.3,
            max_tokens=2048,
        ))

        import json

        result = json.loads(response.text)

        # Validate required fields
        self._validate_result(result)

        return result

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
