"""Deterministic red-flag screening for one consultation turn."""

from __future__ import annotations

from typing import Any

from langgraph.types import StreamWriter

from ..services.red_flag_detector import get_red_flag_detector
from .consultation_thread_prompt import _get_conversation_text
from .consultation_thread_state import ConsultationThreadState


async def safety_check(state: ConsultationThreadState, *, writer: StreamWriter) -> dict[str, Any]:
    detector = get_red_flag_detector()
    red_flag_result = detector.detect(
        state.get("extracted_symptoms", []),
        _get_conversation_text(state),
    )

    if red_flag_result.has_red_flags:
        writer({"type": "red_flag", **red_flag_result.to_dict()})

    return {
        "red_flag_result": red_flag_result.to_dict() if red_flag_result.has_red_flags else None,
    }
