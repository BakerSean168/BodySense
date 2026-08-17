"""Typed Assessment Agent application service."""

from __future__ import annotations

import base64
import binascii
from collections.abc import Callable
from typing import Any

from pydantic_ai import Agent, BinaryContent
from pydantic_ai.messages import UserContent
from pydantic_ai.models import Model

from ..agents.assessment_agent import create_assessment_agent
from ..ai.pydantic_model import get_pydantic_model, route_model_settings
from ..models.assessment import AssessmentAgentOutput, AssessmentDependencies
from ..prompts.assessment import get_assessment_prompt
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_assessment_model,
)

ModelResolver = Callable[[str], Model]


class AssessmentService:
    def __init__(
        self,
        *,
        assessment_agent: Agent[AssessmentDependencies, AssessmentAgentOutput] | None = None,
        model_resolver: ModelResolver | None = None,
    ) -> None:
        self._agent = assessment_agent or create_assessment_agent()
        self._model_resolver = (
            model_resolver
            if model_resolver is not None
            else (None if assessment_agent is not None else get_pydantic_model)
        )

    async def generate_assessment(
        self,
        profile: dict[str, Any],
        rag_context: str = "",
        images: list[str] | None = None,
        posture_analysis: dict[str, Any] | None = None,
        use_case: str = "llm.json",
    ) -> dict[str, Any]:
        deps = AssessmentDependencies(
            profile=profile,
            posture_analysis=posture_analysis or {},
            rag_context=rag_context,
        )
        prompt = get_assessment_prompt(
            profile,
            rag_context,
            posture_analysis=posture_analysis,
        )
        content: list[UserContent] = [prompt]
        for image in images or []:
            if image:
                content.append(_decode_image(image))

        kwargs: dict[str, Any] = {"deps": deps}
        if self._model_resolver is not None:
            kwargs["model"] = self._model_resolver(use_case)
            kwargs["model_settings"] = route_model_settings(use_case)
        result = await self._agent.run(content, **kwargs)
        return result.output.model_dump(mode="json")


def _decode_image(value: str) -> BinaryContent:
    if not value.startswith("data:") or ";base64," not in value:
        raise ValueError("assessment image must be a base64 data URL")
    header, payload = value.split(",", 1)
    media_type = header[5:].split(";", 1)[0]
    if not media_type.startswith("image/"):
        raise ValueError("assessment attachment must be an image")
    try:
        data = base64.b64decode(payload, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise ValueError("assessment image contains invalid base64 data") from exc
    if not data:
        raise ValueError("assessment image is empty")
    return BinaryContent(data=data, media_type=media_type)


_assessment_service: AssessmentService | None = None


def get_assessment_service() -> AssessmentService:
    global _assessment_service
    if _assessment_service is None:
        if deterministic_ai_enabled():
            _assessment_service = AssessmentService(
                assessment_agent=create_assessment_agent(deterministic_assessment_model())
            )
        else:
            _assessment_service = AssessmentService(model_resolver=get_pydantic_model)
    return _assessment_service
