"""Single governance seam for structured AI outputs on the live path.

Diagnosis, treatment, posture, and Assessment leave Python only after passing through
``guard_structured_output``. Callers must not invent parallel policy ifs —
this module is the forced gate before emit/persist.

Diagnosis governance v3 deliberately scans the broad clinical serialization used
by its historical artifacts. Diagnosis governance v4 narrows only the
post-agent Diagnosis scan to fields that assert something about the current
user; candidate names, ``typical_symptoms``, and ``differential`` remain
generic candidate education. The v3 projection is retained for replay and
historical qualification.

Hard gates (per P2 risk note):
- schema validation failures (missing required structure) → rejected
- red-flag hits on *clinical claim content* for diagnosis/treatment → rejected.
  Safety metadata fields (``warning_signs``, ``red_flags``, ``disclaimer``,
  ``citations``) are excluded from the scan so intentional caution text does
  not false-trigger the gate.

Soft gate:
- faithfulness issues → degraded only (substring matching is too weak to hard-block)
- posture red-flag hits → degraded (lower confidence; still surface findings + flags)

Implementation is split by responsibility across sibling modules; this module
keeps the stable public surface and the forced gate entry point:

- ``governance_policy``: policy revisions, detector mapping, required fields,
  safety fallback text
- ``governance_scan``: v3 broad and v4 current-claim serialization
- ``governance_types``: the ``GuardedOutput`` result type
- ``governance_issues``: issue collection and verdict mapping
"""

from __future__ import annotations

import logging
from typing import Any

from ..models.safety import SafetyEnvelopeV2
from ..services.assessment_evidence import AssessmentEvidenceItem
from ..services.governance.output_guard import AIOutputGuard
from ..services.governance.types import GovernanceStatus
from .governance_issues import _collect_issues, _decide_verdict
from .governance_policy import (
    _SAFETY_FALLBACK,
    ASSESSMENT_GOVERNANCE_POLICY_REVISION,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
    DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY,
    TREATMENT_GOVERNANCE_POLICY_REVISION,
    OutputKind,
)
from .governance_types import GuardedOutput

logger = logging.getLogger(__name__)

__all__ = [
    "ASSESSMENT_GOVERNANCE_POLICY_REVISION",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7",
    "DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8",
    "DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY",
    "TREATMENT_GOVERNANCE_POLICY_REVISION",
    "GuardedOutput",
    "OutputKind",
    "guard_structured_output",
]


def guard_structured_output(
    kind: OutputKind,
    payload: dict[str, Any],
    *,
    rag_results: list[dict[str, Any]] | None = None,
    extracted_info: list[dict[str, Any]] | None = None,
    policy_revision: str | None = None,
    assessment_evidence_catalog: dict[str, AssessmentEvidenceItem] | None = None,
    body_state: dict[str, Any] | None = None,
    safety_envelope: SafetyEnvelopeV2 | None = None,
) -> GuardedOutput:
    """Force-gate a structured diagnosis, treatment, posture, or Assessment payload."""
    effective_policy_revision = policy_revision
    if kind == "diagnosis" and policy_revision is not None:
        if policy_revision not in {
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
        }:
            raise ValueError(f"unsupported Diagnosis governance policy revision: {policy_revision}")
    if kind == "diagnosis" and effective_policy_revision is None:
        effective_policy_revision = DIAGNOSIS_GOVERNANCE_POLICY_REVISION
    if kind == "treatment" and policy_revision is not None:
        if policy_revision != TREATMENT_GOVERNANCE_POLICY_REVISION:
            raise ValueError(f"unsupported Treatment governance policy revision: {policy_revision}")
    if kind == "assessment" and policy_revision is not None:
        if policy_revision != ASSESSMENT_GOVERNANCE_POLICY_REVISION:
            raise ValueError(
                f"unsupported Assessment governance policy revision: {policy_revision}"
            )
    if not isinstance(payload, dict):
        return GuardedOutput(
            verdict="rejected",
            kind=kind,
            payload=None,
            reasons=["payload is not a structured object"],
            safety_fallback=_SAFETY_FALLBACK[kind],
        )

    issues = _collect_issues(
        kind,
        payload,
        policy_revision=effective_policy_revision,
        rag_results=rag_results,
        extracted_info=extracted_info,
        assessment_evidence_catalog=assessment_evidence_catalog,
        body_state=body_state,
        safety_envelope=safety_envelope,
    )
    status = _decide_verdict(kind, issues)
    reasons = [i.message for i in issues]
    issue_dicts = [i.to_dict() for i in issues]

    # Keep AIOutputGuard in the call graph so the seam stays one surface.
    _ = AIOutputGuard()

    if status == GovernanceStatus.REJECTED:
        logger.warning(
            "governance rejected %s output: %s",
            kind,
            "; ".join(reasons) or "unspecified",
        )
        return GuardedOutput(
            verdict="rejected",
            kind=kind,
            payload=None,
            reasons=reasons,
            issues=issue_dicts,
            safety_fallback=_SAFETY_FALLBACK[kind],
        )

    if status == GovernanceStatus.DEGRADED:
        logger.info(
            "governance degraded %s output: %s",
            kind,
            "; ".join(reasons) or "unspecified",
        )
        return GuardedOutput(
            verdict="degraded",
            kind=kind,
            payload=payload,
            reasons=reasons,
            issues=issue_dicts,
        )

    return GuardedOutput(
        verdict="accepted",
        kind=kind,
        payload=payload,
        reasons=reasons,
        issues=issue_dicts,
    )
