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
from ..models.assessment import (
    ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
    AssessmentDependencies,
)
from ..prompts.assessment import ASSESSMENT_DISCLAIMER, get_assessment_prompt
from ..runtime.governance import guard_structured_output
from ..testing_support.deterministic_ai import (
    deterministic_ai_enabled,
    deterministic_assessment_model,
)
from .assessment_evidence import (
    build_assessment_evidence_catalog,
    build_assessment_evidence_coverage,
    build_assessment_evidence_gaps,
    build_assessment_summary,
    derive_assessment_status,
    evidence_catalog_for_prompt,
    render_assessment_observations,
)

ModelResolver = Callable[[AssessmentAgentManifest], Model]


class AssessmentOutputRejectedError(ValueError):
    """A model output failed the deterministic Assessment governance boundary."""


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
        reviewed_report_evidence: list[Any] | None = None,
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
        normalized_body_state = body_state or {}
        normalized_indicators = report_indicators or []
        normalized_reviewed = reviewed_report_evidence or []
        normalized_posture = posture_analysis or {}
        normalized_images = [value for value in (images or []) if value]

        if (
            config.output_schema_revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2
            and normalized_images
        ):
            raise ValueError(
                "assessment-output-v2 does not accept raw images; run Posture analysis first"
            )
        if (
            config.output_schema_revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2
            and rag_context.strip()
        ):
            raise ValueError("assessment-output-v2 does not accept unmodeled rag_context evidence")

        evidence_catalog = build_assessment_evidence_catalog(
            profile=profile,
            body_state=normalized_body_state,
            report_indicators=normalized_indicators,
            reviewed_report_evidence=normalized_reviewed,
            posture_analysis=normalized_posture,
            evidence_policy_revision=config.evidence_policy_revision,
        )
        if (
            config.output_schema_revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2
            and not evidence_catalog
        ):
            coverage = build_assessment_evidence_coverage(evidence_catalog)
            payload: dict[str, Any] = {
                "contract_revision": ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
                "status": derive_assessment_status(coverage),
                "evidence_policy_revision": config.evidence_policy_revision,
                "observations": [],
                "evidence_coverage": coverage,
                "evidence_gaps": build_assessment_evidence_gaps(coverage),
                "summary": build_assessment_summary(0, coverage),
                "safety_notes": [ASSESSMENT_DISCLAIMER],
                "governance": {
                    "verdict": "accepted",
                    "policy_revision": config.governance_policy_revision,
                    "issues": [],
                },
                "agent_configuration": config.provenance(),
                "execution_provenance": _skipped_execution_provenance(config),
            }
            return payload

        deps = AssessmentDependencies(
            profile=profile,
            body_state=normalized_body_state,
            report_indicators=normalized_indicators,
            reviewed_report_evidence=normalized_reviewed,
            posture_analysis=normalized_posture,
            rag_context=rag_context,
            evidence_catalog=evidence_catalog_for_prompt(evidence_catalog),
        )
        prompt = get_assessment_prompt(
            profile,
            rag_context,
            posture_analysis=normalized_posture,
            prompt_revision=config.prompt_revision,
        )
        content: list[UserContent] = [prompt]
        if config.output_schema_revision != ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2:
            for image in normalized_images:
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
        model_payload = result.output.model_dump(mode="json")

        if config.output_schema_revision == ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2:
            guarded = guard_structured_output(
                "assessment",
                model_payload,
                policy_revision=config.governance_policy_revision,
                assessment_evidence_catalog=evidence_catalog,
            )
            if guarded.verdict == "rejected" or guarded.payload is None:
                raise AssessmentOutputRejectedError(
                    "Assessment output rejected by evidence governance"
                )
            selections = list(guarded.payload.get("observations") or [])
            observations = render_assessment_observations(selections, evidence_catalog)
            coverage = build_assessment_evidence_coverage(evidence_catalog)
            payload: dict[str, Any] = {
                "contract_revision": ASSESSMENT_OUTPUT_SCHEMA_REVISION_V2,
                "status": derive_assessment_status(coverage),
                "evidence_policy_revision": config.evidence_policy_revision,
                "observations": observations,
                "evidence_coverage": coverage,
                "evidence_gaps": build_assessment_evidence_gaps(coverage),
                "summary": build_assessment_summary(len(observations), coverage),
                "safety_notes": [ASSESSMENT_DISCLAIMER],
                "governance": {
                    "verdict": guarded.verdict,
                    "policy_revision": config.governance_policy_revision,
                    "issues": guarded.issues,
                },
            }
        else:
            # Immutable v1/v2 historical configs keep their original output
            # shape for read-only counterfactual replay. Go serving policy no
            # longer allows these legacy contracts to create durable reports.
            payload = model_payload

        payload["agent_configuration"] = config.provenance()
        payload["execution_provenance"] = _execution_provenance(result, config)
        return payload


def _skipped_execution_provenance(config: AssessmentAgentManifest) -> dict[str, Any]:
    """Provenance for a deterministic no-evidence Assessment derivation."""

    return {
        "status": "skipped_no_evidence",
        "runtime": "deterministic",
        "logical_model": config.logical_model,
        "model_group_revision": config.model_group_revision,
        "gateway_reported_model": None,
        "provider_adapter": None,
        "agent_run_id": None,
        "usage": {
            "requests": 0,
            "input_tokens": 0,
            "output_tokens": 0,
            "total_tokens": 0,
        },
    }


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
