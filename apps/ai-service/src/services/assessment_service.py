"""Typed Assessment Agent application service."""

from __future__ import annotations

import base64
import binascii
from collections.abc import Callable
from typing import Any

from pydantic_ai import BinaryContent
from pydantic_ai.messages import UserContent
from pydantic_ai.models import Model

from ..agents.assessment_agent import create_assessment_agent
from ..ai.assessment_gateway_model import (
    assessment_model_settings,
    get_assessment_runtime_model,
)
from ..configuration.assessment_agent_config import (
    AssessmentAgentManifest,
    get_assessment_configuration,
    get_default_assessment_configuration,
)
from ..models.assessment import AssessmentDependencies
from ..prompts.assessment import get_assessment_prompt
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_assessment_model,
)

ModelResolver = Callable[[AssessmentAgentManifest], Model]


class AssessmentService:
    def __init__(
        self,
        *,
        model_resolver: ModelResolver | None = None,
    ) -> None:
        self._model_resolver = model_resolver or get_assessment_runtime_model

    async def generate_assessment(
        self,
        profile: dict[str, Any],
        body_state: dict[str, Any] | None = None,
        report_indicators: list[Any] | None = None,
        rag_context: str = "",
        images: list[str] | None = None,
        posture_analysis: dict[str, Any] | None = None,
        configuration_id: str | None = None,
    ) -> dict[str, Any]:
        config = (
            get_assessment_configuration(configuration_id)
            if configuration_id
            else get_default_assessment_configuration()
        )
        deps = AssessmentDependencies(
            profile=profile,
            body_state=body_state or {},
            report_indicators=report_indicators or [],
            posture_analysis=posture_analysis or {},
            rag_context=rag_context,
        )
        prompt = get_assessment_prompt(
            profile,
            rag_context,
            posture_analysis=posture_analysis,
            prompt_revision=config.prompt_revision,
        )
        content: list[UserContent] = [prompt]
        for image in images or []:
            if image:
                content.append(_decode_image(image))

        agent = create_assessment_agent(
            prompt_revision=config.prompt_revision,
            output_schema_revision=config.output_schema_revision,
            tool_policy_revision=config.tool_policy_revision,
        )
        run_kwargs: dict[str, Any] = {
            "deps": deps,
            "model": self._model_resolver(config),
            "model_settings": assessment_model_settings(config),
        }
        result = await agent.run(content, **run_kwargs)
        payload = result.output.model_dump(mode="json")
        payload["agent_configuration"] = config.provenance()
        payload["execution_provenance"] = _execution_provenance(result, config)
        return payload


def _execution_provenance(result: Any, config: AssessmentAgentManifest) -> dict[str, Any]:
    response = result.response
    usage = result.usage
    return {
        "status": "executed",
        "runtime": "pydantic-ai",
        "logical_model": config.logical_model,
        "model_group_revision": config.model_group_revision,
        "gateway_reported_model": response.model_name,
        "provider_adapter": response.provider_name,
        "agent_run_id": str(response.run_id) if response.run_id is not None else None,
        "usage": {
            "requests": usage.requests,
            "input_tokens": usage.input_tokens,
            "output_tokens": usage.output_tokens,
            "total_tokens": (usage.input_tokens or 0) + (usage.output_tokens or 0),
        },
    }


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
                model_resolver=lambda _config: deterministic_assessment_model()
            )
        else:
            _assessment_service = AssessmentService()
    return _assessment_service
