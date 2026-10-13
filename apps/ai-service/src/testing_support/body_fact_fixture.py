"""Synthetic known corpus for the explicitly enabled E2E runtime only.

The actual PydanticAI output validator and production extraction node still run.
This is not a language-understanding benchmark or a production heuristic fallback.
"""

from __future__ import annotations

from typing import Any

from pydantic_ai.models.test import TestModel

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..runtime.body_fact_capture import acquire_body_fact_turn
from ..services.body_fact_intake_service import BodyFactIntakeService


async def body_fact_fixture_events(
    text: str,
    body_state: dict[str, Any],
    manifest: ConsultationAgentManifest,
) -> list[dict[str, Any]]:
    corpus = {
        "我每晚睡七小时。": ("lifestyle", "unknown"),
        "我昨天肩膀不舒服。": ("symptom", "present"),
        "我没有肩痛。": ("symptom", "absent"),
    }
    args: dict[str, Any] = {"turn_kind": "other", "facts": []}
    if text in corpus:
        kind, presence = corpus[text]
        args = {
            "turn_kind": "self_report",
            "facts": [
                {
                    "kind": kind,
                    "statement": text,
                    "subject": "self",
                    "presence": presence,
                }
            ],
        }
    service = BodyFactIntakeService(model_resolver=lambda _: TestModel(custom_output_args=args))
    events: list[dict[str, Any]] = []
    await acquire_body_fact_turn(
        {"latest_user_message": text, "body_state": body_state},
        writer=events.append,
        manifest=manifest,
        intake_service=service,
    )
    return events
