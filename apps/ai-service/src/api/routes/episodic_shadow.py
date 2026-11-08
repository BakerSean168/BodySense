"""Internal S2-B shadow proposal transport.

Go selects and locks the source window, calls this route only while its
default-off EPISODIC_SHADOW_GENERATION_ENABLED switch is on, and re-verifies
every quote before persistence. This adds no public surface: it sits on the
same internal router as /runtime/threads, and the payload can never name a
note, retention decision or clinical authority.
"""

from __future__ import annotations

import logging
from typing import Any

from fastapi import APIRouter, HTTPException
from pydantic_ai.models import Model

from ...ai.errors import GatewayUnavailableError
from ...ai.gateway import get_gateway_model
from ...services.episodic_shadow_proposal import (
    InvalidShadowProposalError,
    ShadowGenerationDisabledError,
)
from ..episodic_shadow_proto_adapter import parse_shadow_source_window, propose_shadow_quote_proto

logger = logging.getLogger(__name__)

# The runtime's general text logical model group. Model construction reuses the
# existing gateway path (LITELLM_BASE_URL/LITELLM_API_KEY, OpenAI compatible)
# instead of a second provider or a route-level generation setting.
SHADOW_QUOTE_LOGICAL_MODEL = "bodysense-text"

router = APIRouter(prefix="/runtime/episodic-shadow", tags=["runtime"])


def _shadow_quote_model() -> Model:
    return get_gateway_model(SHADOW_QUOTE_LOGICAL_MODEL)


@router.post("/proposals")
async def propose_episodic_shadow_quote(payload: dict[str, Any]) -> dict[str, object]:
    try:
        # Adapter validation runs before model resolution: an invalid or
        # out-of-window command is rejected as-is and never reaches a model.
        parse_shadow_source_window(payload)
        return await propose_shadow_quote_proto(payload, model=_shadow_quote_model(), enabled=True)
    except InvalidShadowProposalError as exc:
        logger.warning("Rejected invalid shadow proposal command: %s", exc)
        raise HTTPException(
            status_code=422, detail="invalid private shadow proposal command"
        ) from exc
    except (ShadowGenerationDisabledError, GatewayUnavailableError) as exc:
        logger.warning("Shadow proposal generation unavailable: %s", exc)
        raise HTTPException(
            status_code=503, detail="shadow proposal generation unavailable"
        ) from exc
    except Exception as exc:
        logger.exception("Episodic shadow proposal failed")
        raise HTTPException(status_code=500, detail="episodic shadow proposal failed") from exc
