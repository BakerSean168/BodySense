"""Project a governed Diagnosis output onto the Go formal-analysis contract.

The ``diagnosis-agent-proposals-v1`` block is the only part of a Diagnosis
response that Go ingests into a formal run. It carries Agent judgment only:
no durable IDs, owner, revision, evidence version, disposition or user fact.

Go still validates the block strictly and fails closed. This projection repairs
only recoverable gaps in model output so a usable analysis is not thrown away:
a missing assessment level becomes the weaker ``suspected`` (``likely`` only
when the model said high confidence), empty summaries are filled from the
model's own reasoning or a cautious default, evidence references that are not
in the frozen BodyState are dropped, and a candidate left without a name or
without supporting evidence is dropped. No candidate left means
``insufficient_evidence`` with zero proposals.
"""

from __future__ import annotations

from typing import Any

FORMAL_PROPOSALS_SCHEMA_REVISION = "diagnosis-agent-proposals-v1"
_PROPOSAL_STATUSES = frozenset({"completed", "partial"})
_ACCEPTED_VERDICTS = frozenset({"accepted", "degraded"})
_LEVELS = frozenset({"suspected", "likely"})
_MAX_TEXT = 4000
_MAX_ITEMS = 50
_MAX_EVIDENCE = 200
_DEFAULT_CAUSE = "可能成因尚不明确，需要更多信息进一步评估。"
_DEFAULT_HANDLING = "记录症状变化；如持续、加重或出现警示信号，请及时寻求专业评估。"


def _ids(body_state: dict[str, Any], key: str) -> set[str]:
    items = body_state.get(key) or []
    return {str(item.get("id")) for item in items if isinstance(item, dict) and item.get("id")}


def _text(value: Any) -> str:
    return value.strip()[:_MAX_TEXT] if isinstance(value, str) else ""


def _first(*values: Any) -> str:
    for value in values:
        text = _text(value)
        if text:
            return text
    return ""


def _strings(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [t for t in (_text(item) for item in value) if t][:_MAX_ITEMS]


def _level(candidate: dict[str, Any]) -> str:
    level = _text(candidate.get("assessment_level"))
    if level in _LEVELS:
        return level
    return "likely" if _text(candidate.get("confidence")) in {"高", "high"} else "suspected"


def _evidence(candidate: dict[str, Any], facts: set[str], observations: set[str]) -> list[dict]:
    rationale = _first(
        candidate.get("basis"), candidate.get("reasoning_summary"), candidate.get("name")
    )
    evidence: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()

    def add(raw_id: Any, role: str) -> None:
        evidence_id = str(raw_id).strip()
        if evidence_id in facts:
            evidence_type = "body_fact"
        elif evidence_id in observations:
            evidence_type = "body_observation"
        else:
            return  # not in the frozen input: drop rather than reject the run
        if (evidence_id, role) in seen:
            return
        seen.add((evidence_id, role))
        evidence.append(
            {
                "evidenceType": evidence_type,
                "evidenceId": evidence_id,
                "role": role,
                "rationale": rationale,
            }
        )

    for key in ("basis_fact_ids", "basis_observation_ids", "supporting_evidence_ids"):
        for raw in candidate.get(key) or []:
            add(raw, "supporting")
    for raw in candidate.get("counterevidence_ids") or []:
        add(raw, "counter")
    return evidence[:_MAX_EVIDENCE]


def _proposal(candidate: dict[str, Any], facts: set[str], observations: set[str]) -> dict | None:
    name = _text(candidate.get("name"))[:240]
    evidence = _evidence(candidate, facts, observations)
    if not name or not any(e["role"] == "supporting" for e in evidence):
        return None
    differential = _text(candidate.get("differential"))
    rationale = _first(candidate.get("reasoning_summary"), candidate.get("basis"), name)
    actions = _strings(candidate.get("handling_actions"))
    return {
        "changeKind": "create",
        "medicalName": name,
        "assessment": {
            "level": _level(candidate),
            "rationale": rationale,
            "informationGaps": _strings(candidate.get("missing_information")),
            "differentialNotes": [differential] if differential else [],
        },
        "causeAnalysis": {
            "summary": _first(candidate.get("cause_summary"), _DEFAULT_CAUSE),
            "rationale": _first(candidate.get("basis"), rationale),
        },
        "informationGaps": _strings(candidate.get("missing_information")),
        "handlingPlan": {
            "summary": _first(candidate.get("handling_summary"), _DEFAULT_HANDLING),
            "actions": actions,
            "cautionNotes": _strings(candidate.get("caution_notes")),
        },
        "evidence": evidence,
    }


def _insufficient(summary: str) -> dict:
    return {
        "schemaRevision": FORMAL_PROPOSALS_SCHEMA_REVISION,
        "outcome": "insufficient_evidence",
        "summary": summary,
        "proposals": [],
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
        return _insufficient(summary)
    facts = _ids(body_state, "facts")
    observations = _ids(body_state, "observations")
    proposals: list[dict] = []
    names: set[str] = set()
    for candidate in candidates:
        proposal = _proposal(candidate, facts, observations)
        if proposal is None:
            continue
        key = " ".join(proposal["medicalName"].lower().split())
        if key in names:
            continue
        names.add(key)
        proposals.append(proposal)
        if len(proposals) == 50:
            break
    if not proposals:
        return _insufficient(summary)
    return {
        "schemaRevision": FORMAL_PROPOSALS_SCHEMA_REVISION,
        "outcome": "proposals",
        "summary": summary,
        "proposals": proposals,
    }
