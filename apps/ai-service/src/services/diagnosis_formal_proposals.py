"""Project a governed Diagnosis output onto the Go formal-analysis contract.

The ``diagnosis-agent-proposals-v1`` block is the only part of a Diagnosis
response that Go ingests into a formal run. It carries Agent judgment only:
no durable IDs, owner, revision, evidence version, disposition or user fact.
Go validates it strictly and rejects the whole output on any defect, so this
projection never repairs, guesses or drops Agent content to make it pass.
"""

from __future__ import annotations

from typing import Any

FORMAL_PROPOSALS_SCHEMA_REVISION = "diagnosis-agent-proposals-v1"
_PROPOSAL_STATUSES = frozenset({"completed", "partial"})
_ACCEPTED_VERDICTS = frozenset({"accepted", "degraded"})
# Unresolvable references are passed through under this type so Go fails closed.
UNRESOLVED_EVIDENCE_TYPE = "unresolved"


def _ids(body_state: dict[str, Any], key: str) -> set[str]:
    items = body_state.get(key) or []
    return {str(item.get("id")) for item in items if isinstance(item, dict) and item.get("id")}


def _text(value: Any) -> str:
    return value if isinstance(value, str) else ""


def _strings(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, str)]


def _evidence(candidate: dict[str, Any], facts: set[str], observations: set[str]) -> list[dict]:
    rationale = (
        _text(candidate.get("basis"))
        or _text(candidate.get("reasoning_summary"))
        or _text(candidate.get("name"))
    )
    evidence: list[dict[str, Any]] = []

    def add(raw_id: Any, role: str, declared: str | None) -> None:
        evidence_id = str(raw_id)
        if declared is not None:
            evidence_type = declared
        elif evidence_id in facts:
            evidence_type = "body_fact"
        elif evidence_id in observations:
            evidence_type = "body_observation"
        else:
            evidence_type = UNRESOLVED_EVIDENCE_TYPE
        evidence.append(
            {
                "evidenceType": evidence_type,
                "evidenceId": evidence_id,
                "role": role,
                "rationale": rationale,
            }
        )

    for raw in candidate.get("basis_fact_ids") or []:
        add(raw, "supporting", "body_fact")
    for raw in candidate.get("basis_observation_ids") or []:
        add(raw, "supporting", "body_observation")
    for raw in candidate.get("counterevidence_ids") or []:
        add(raw, "counter", None)
    return evidence


def _proposal(candidate: dict[str, Any], facts: set[str], observations: set[str]) -> dict:
    differential = _text(candidate.get("differential"))
    rationale = _text(candidate.get("reasoning_summary")) or _text(candidate.get("basis"))
    return {
        "changeKind": "create",
        "medicalName": _text(candidate.get("name")),
        "assessment": {
            "level": _text(candidate.get("assessment_level")),
            "rationale": rationale,
            "informationGaps": _strings(candidate.get("missing_information")),
            "differentialNotes": [differential] if differential else [],
        },
        "causeAnalysis": {
            "summary": _text(candidate.get("cause_summary")),
            "rationale": _text(candidate.get("basis")) or rationale,
        },
        "informationGaps": _strings(candidate.get("missing_information")),
        "handlingPlan": {
            "summary": _text(candidate.get("handling_summary")),
            "actions": _strings(candidate.get("handling_actions")),
            "cautionNotes": _strings(candidate.get("caution_notes")),
        },
        "evidence": _evidence(candidate, facts, observations),
    }


def build_formal_proposals(payload: dict[str, Any], body_state: dict[str, Any]) -> dict:
    """Return the formal proposal block for one governed Diagnosis payload.

    Safety and adoption remain Go decisions; a non-proposal status or a rejected
    governance verdict yields ``insufficient_evidence`` with zero proposals.
    """

    governance = payload.get("governance")
    verdict = governance.get("verdict") if isinstance(governance, dict) else None
    candidates = [c for c in payload.get("candidates") or [] if isinstance(c, dict)]
    summary = _text(payload.get("summary"))
    if (
        payload.get("status") not in _PROPOSAL_STATUSES
        or verdict not in _ACCEPTED_VERDICTS
        or not candidates
    ):
        return {
            "schemaRevision": FORMAL_PROPOSALS_SCHEMA_REVISION,
            "outcome": "insufficient_evidence",
            "summary": summary,
            "proposals": [],
        }
    facts = _ids(body_state, "facts")
    observations = _ids(body_state, "observations")
    return {
        "schemaRevision": FORMAL_PROPOSALS_SCHEMA_REVISION,
        "outcome": "proposals",
        "summary": summary,
        "proposals": [_proposal(c, facts, observations) for c in candidates],
    }
