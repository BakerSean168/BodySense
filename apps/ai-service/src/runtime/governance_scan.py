"""Claim-surface serialization used by the governance red-flag scan.

v3 serializes the historical broad clinical surface unchanged. v4 and later
narrow the post-agent Diagnosis scan to fields that assert something about the
current user; candidate education fields stay generic by design.
"""

from __future__ import annotations

import json
from typing import Any

# Fields that hold safety / provenance metadata rather than clinical claims.
_RED_FLAG_SCAN_EXCLUDE = frozenset(
    {
        "warning_signs",
        "red_flags",
        "disclaimer",
        "citations",
        "faithfulness",
        "governance",
        "safety_note",
        "safety_notes",
        "safety_summary",
        "review_triggers",
        "stop_conditions",
        "missing_information",
        "safety_fallback",
    }
)

# Diagnosis v4 treats these candidate fields as education about a possibility,
# not assertions about the current user. In particular, ``differential`` stays
# on this non-user-claim surface: it explains how a candidate differs from
# nearby possibilities and may mention the red-flag concepts that distinguish
# them. Unknown fields remain scan-visible so a new current-claim field cannot
# silently bypass the safety gate.
_DIAGNOSIS_V4_CANDIDATE_EDUCATION_FIELDS = frozenset({"name", "typical_symptoms", "differential"})

# These fields carry provenance, authority, or acquisition metadata. The v3
# serializer intentionally does not use this set; changing it there would
# change the historical v3 behavior.
_DIAGNOSIS_V4_METADATA_FIELDS = _RED_FLAG_SCAN_EXCLUDE | frozenset(
    {
        "agent_configuration",
        "decision_authority",
        "evidence_acquisition",
        "execution_provenance",
        "rollout_provenance",
    }
)


def _clinical_claim_text(payload: dict[str, Any]) -> str:
    """Serialize the historical v3 clinical claim surface unchanged."""

    def _strip(value: Any) -> Any:
        if isinstance(value, dict):
            return {
                key: _strip(item)
                for key, item in value.items()
                if key not in _RED_FLAG_SCAN_EXCLUDE
            }
        if isinstance(value, list):
            return [_strip(item) for item in value]
        return value

    return json.dumps(_strip(payload), ensure_ascii=False)


def _diagnosis_v4_current_claim_text(payload: dict[str, Any]) -> str:
    """Serialize only Diagnosis fields that assert current-user clinical facts.

    ``typical_symptoms`` and ``differential`` describe candidate education, not
    the user's present state, so v4 intentionally excludes them from this
    current-claim scan. Candidate ``basis``, ``impact`` and
    ``reasoning_summary`` are current-user claims and remain scanned.
    """

    def _strip_metadata(value: Any) -> Any:
        if isinstance(value, dict):
            return {
                key: _strip_metadata(item)
                for key, item in value.items()
                if key not in _RED_FLAG_SCAN_EXCLUDE
            }
        if isinstance(value, list):
            return [_strip_metadata(item) for item in value]
        return value

    claims: dict[str, Any] = {}
    for field_name, value in payload.items():
        if field_name in _DIAGNOSIS_V4_METADATA_FIELDS:
            continue
        if field_name != "candidates":
            claims[field_name] = _strip_metadata(value)
            continue

        if not isinstance(value, list):
            claims[field_name] = _strip_metadata(value)
            continue

        current_claims: list[Any] = []
        for candidate in value:
            if not isinstance(candidate, dict):
                current_claims.append(_strip_metadata(candidate))
                continue
            current_claims.append(
                {
                    field_name: _strip_metadata(field_value)
                    for field_name, field_value in candidate.items()
                    if (
                        field_name not in _DIAGNOSIS_V4_CANDIDATE_EDUCATION_FIELDS
                        and field_name not in _RED_FLAG_SCAN_EXCLUDE
                    )
                }
            )
        claims[field_name] = current_claims

    return json.dumps(claims, ensure_ascii=False)
