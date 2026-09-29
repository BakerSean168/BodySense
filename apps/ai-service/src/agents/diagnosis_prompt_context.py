"""Pure model-facing views; frozen inputs and governance retain full authority."""

from __future__ import annotations

import json
from copy import deepcopy
from typing import Any

SAFETY_DETAIL_KEYS = frozenset(
    {
        "safety_capture_revision",
        "trauma",
        "radiating_pain",
        "numbness",
        "weakness",
        "dizziness",
    }
)

_FACT_CHANGE_PAYLOAD_KEYS = frozenset(
    {"fact", "before", "after", "previous", "replacement", "candidate"}
)


def _sanitize_fact(fact: Any) -> Any:
    """Copy one fact payload and remove only successor-owned authority details."""

    copied = deepcopy(fact)
    if not isinstance(copied, dict):
        return copied
    details = copied.get("details")
    if isinstance(details, dict):
        copied["details"] = {
            key: value for key, value in details.items() if key not in SAFETY_DETAIL_KEYS
        }
    return copied


def _sanitize_fact_revision(row: Any, *, remove_transport_ids: bool) -> Any:
    """Copy one BodyState revision and sanitize direct fact payload slots only."""

    copied = deepcopy(row)
    if not isinstance(copied, dict):
        return copied

    if remove_transport_ids:
        copied.pop("id", None)
        copied.pop("user_id", None)

    change_type = str(copied.get("change_type", ""))
    changes = copied.get("changes")
    if not isinstance(changes, dict):
        return copied

    if change_type.startswith("fact.") or change_type.startswith("fact_"):
        for key in _FACT_CHANGE_PAYLOAD_KEYS:
            if key in changes and isinstance(changes[key], dict):
                changes[key] = _sanitize_fact(changes[key])
        return copied

    if change_type == "current_context.updated":
        fact_changes = changes.get("facts")
        if isinstance(fact_changes, list):
            sanitized_changes = []
            for item in fact_changes:
                item_copy = deepcopy(item)
                if isinstance(item_copy, dict):
                    for key in _FACT_CHANGE_PAYLOAD_KEYS:
                        if key in item_copy and isinstance(item_copy[key], dict):
                            item_copy[key] = _sanitize_fact(item_copy[key])
                sanitized_changes.append(item_copy)
            changes["facts"] = sanitized_changes
    return copied


def legacy_body_state_prompt_view(body_state: dict[str, Any]) -> dict[str, Any]:
    """Preserve historical serialization while hiding post-version fact authority metadata."""

    view = deepcopy(body_state)
    facts = view.get("facts")
    if isinstance(facts, list):
        view["facts"] = [_sanitize_fact(fact) for fact in facts]
    revisions = view.get("recent_revisions")
    if isinstance(revisions, list):
        view["recent_revisions"] = [
            _sanitize_fact_revision(row, remove_transport_ids=False) for row in revisions
        ]
    return view


def legacy_history_prompt_view(history: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Preserve historical row shape and sanitize only fact revision payloads."""

    return [_sanitize_fact_revision(row, remove_transport_ids=False) for row in history]


def body_state_prompt_view(body_state: dict[str, Any]) -> dict[str, Any]:
    """Return the v9 BodyState model view with duplicate transport context removed."""

    view = deepcopy(body_state)
    view.pop("recent_revisions", None)
    view.pop("user_id", None)
    facts = view.get("facts")
    if isinstance(facts, list):
        view["facts"] = [_sanitize_fact(fact) for fact in facts]
    return view


def history_prompt_view(history: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Return the v9 history view, preserving semantics and sanitizing fact payloads only."""

    return [_sanitize_fact_revision(row, remove_transport_ids=True) for row in history]


def profile_prompt_view(profile: dict[str, Any]) -> dict[str, Any]:
    return deepcopy(profile)


def compact_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
