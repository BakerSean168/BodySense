"""Posture photo analysis service (Phase 1: pure VLM).

Sends a single-view photo to a vision model and returns a governed, structured
posture analysis. Governance guarantees three safety invariants regardless of
what the model returns:

1. No numeric metrics leak through (anti-hallucination — Phase 1 has no geometry).
2. Findings are constrained to the keys judgeable from the given view.
3. A medical disclaimer is always present, and red flags trigger a "see a
   professional" message.
"""

from __future__ import annotations

import base64
import json
import logging

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage
from ..prompts.posture import (
    DEFAULT_DISCLAIMER,
    KEY_LABELS,
    VIEW_ALLOWED_KEYS,
    VIEW_LABEL,
    build_posture_system_prompt,
)
from .governance.output_guard import AIOutputGuard
from .red_flag_detector import get_red_flag_detector

logger = logging.getLogger(__name__)

_REQUIRED_FIELDS = ["view", "findings", "summary_markdown", "disclaimer"]
_VALID_SEVERITY = {"mild", "moderate", "marked"}
_VALID_CONFIDENCE = {"high", "medium", "low"}

# Module-level AIService singleton to avoid re-parsing config on every request.
_ai_service_instance: AIService | None = None


def _get_ai_service() -> AIService:
    global _ai_service_instance
    if _ai_service_instance is None:
        _ai_service_instance = AIService()
    return _ai_service_instance


async def analyze_posture(
    image_bytes: bytes,
    mime_type: str,
    view: str,
    ai: AIService | None = None,
) -> dict:
    """Analyze a single posture photo and return a governed result dict."""
    ai = ai or _get_ai_service()
    b64 = base64.b64encode(image_bytes).decode()
    view_label = VIEW_LABEL.get(view, view)

    messages = [
        ChatMessage(role="system", content=build_posture_system_prompt(view)),
        ChatMessage(
            role="user",
            content=[
                {
                    "type": "text",
                    "text": (
                        f"这是用户的{view_label}站姿照片，"
                        f"请严格按 {view} 视角分析体态并输出规定的 JSON。"
                    ),
                },
                {
                    "type": "image_url",
                    "image_url": {"url": f"data:{mime_type};base64,{b64}"},
                },
            ],
        ),
    ]

    resp = await ai.generate(
        AiRequest(
            use_case="posture.analyze",
            messages=messages,
            response_format="json_object",
        )
    )

    try:
        data = json.loads(resp.text)
    except (json.JSONDecodeError, TypeError):
        logger.warning("posture analysis returned non-JSON output; degrading")
        data = {}

    return govern_posture_result(data, view)


def govern_posture_result(data: dict, view: str) -> dict:
    """Apply all Phase-1 safety invariants to a raw model result.

    This is deterministic and independently testable — it never calls the LLM.
    """
    if not isinstance(data, dict):
        data = {}

    data["schema_version"] = 1
    data["view"] = view

    allowed = set(VIEW_ALLOWED_KEYS.get(view, []))
    cleaned_findings: list[dict] = []
    for f in data.get("findings", []) or []:
        if not isinstance(f, dict):
            continue
        key = f.get("key")
        # Enforce per-view allow-list: drop cross-view guesses.
        if key not in allowed:
            continue
        # Anti-hallucination: Phase 1 never keeps numeric metrics.
        f["metric"] = None
        # Normalize label / severity / confidence.
        f["label"] = f.get("label") or KEY_LABELS.get(key, key)
        if f.get("severity") not in _VALID_SEVERITY:
            f["severity"] = "mild"
        if f.get("confidence") not in _VALID_CONFIDENCE:
            f["confidence"] = "low"
        f.setdefault("evidence", "")
        cleaned_findings.append(f)
    data["findings"] = cleaned_findings

    # Always carry a disclaimer.
    if not data.get("disclaimer"):
        data["disclaimer"] = DEFAULT_DISCLAIMER

    data.setdefault("summary_markdown", "")
    data.setdefault("overall_confidence", "medium")
    data.setdefault("red_flags", [])

    # Red-flag scan over the summary + evidence text (high-recall, reuses the
    # consultation detector). Merge with any model-declared red flags.
    scan_text = data.get("summary_markdown", "") + " " + " ".join(
        f.get("evidence", "") for f in cleaned_findings
    )
    rf = get_red_flag_detector().detect([], scan_text)
    if rf.has_red_flags:
        existing = {
            (r.get("category"), r.get("message"))
            for r in data["red_flags"]
            if isinstance(r, dict)
        }
        for flag in rf.flags:
            item = {"category": flag.category, "message": flag.message}
            if (item["category"], item["message"]) not in existing:
                data["red_flags"].append(item)

    # Structured-output governance: if required fields are missing, degrade the
    # reported confidence rather than fail hard.
    result = AIOutputGuard().validate_structured_output(
        data, required_fields=_REQUIRED_FIELDS
    )
    if result.status.value != "accepted":
        data["overall_confidence"] = "low"

    return data
