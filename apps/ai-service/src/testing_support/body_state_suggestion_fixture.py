"""Synthetic suggestion corpus for the explicitly enabled E2E runtime only.

The production proposer node, its output validator and the span/envelope
mapping still run; only the model is a deterministic TestModel.
"""

from __future__ import annotations

from typing import Any

from pydantic_ai.models.test import TestModel

from ..configuration.consultation_agent_config import ConsultationAgentManifest
from ..runtime.body_state_suggestion import propose_body_state_suggestion
from ..services.body_state_suggestion_service import BodyStateSuggestionService

SUGGESTION_CORPUS: dict[str, dict[str, str]] = {
    "我觉得是久坐导致的。": {"kind": "hypothesis", "quote": "我觉得是久坐导致的"},
    "我早上量血压是135/88 mmHg。": {
        "kind": "observation",
        "quote": "我早上量血压是135/88 mmHg",
        "observation_kind": "blood_pressure",
        "raw_value": "135/88",
        "raw_unit": "mmHg",
    },
}

# Every option a bound suggestion (or its unverified notice) can be answered with.
SUGGESTION_ANSWER_OPTIONS = frozenset(
    {"记录为我的猜测", "记录为身体观察", "记录为一次发作", "不记录", "知道了"}
)


async def body_state_suggestion_fixture(
    text: str,
    body_state: dict[str, Any],
    manifest: ConsultationAgentManifest,
    run_id: str,
    message_id: str | None,
    *,
    pending_revisions: int = 0,
) -> dict[str, Any] | None:
    """Return the typed suggestion the production proposer emits for text."""
    args = {"suggestion": SUGGESTION_CORPUS.get(text)}
    service = BodyStateSuggestionService(
        model_resolver=lambda _: TestModel(custom_output_args=args)
    )
    proposed = await propose_body_state_suggestion(
        {
            "run_id": run_id,
            "latest_user_message": text,
            "latest_user_message_id": message_id or "",
            "body_state": body_state,
            "body_state_pending_revisions": pending_revisions,
        },
        manifest=manifest,
        service=service,
    )
    return proposed["body_state_suggestion"]
