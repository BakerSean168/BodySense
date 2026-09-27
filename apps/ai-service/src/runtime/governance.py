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
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from typing import Any, Literal

from ..services.assessment_evidence import (
    AssessmentEvidenceItem,
    assessment_evidence_issues,
)
from ..services.governance.output_guard import AIOutputGuard
from ..services.governance.policies import check_faithfulness, check_red_flags, check_schema_valid
from ..services.governance.types import (
    GovernanceContext,
    GovernanceIssue,
    GovernanceStatus,
    IssueSeverity,
)
from ..services.red_flag_detector import (
    DEFAULT_RED_FLAG_DETECTOR_REVISION,
    RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
)

logger = logging.getLogger(__name__)

OutputKind = Literal["diagnosis", "treatment", "posture", "assessment"]

DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3 = "diagnosis-governance-v3"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4 = "diagnosis-governance-v4-claim-surface"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5 = "diagnosis-governance-v5-negation-aware-claims"
DIAGNOSIS_GOVERNANCE_POLICY_REVISION = DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4
TREATMENT_GOVERNANCE_POLICY_REVISION = "treatment-governance-v1"
ASSESSMENT_GOVERNANCE_POLICY_REVISION = "assessment-governance-v2"

DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY = {
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4: RED_FLAG_DETECTOR_REVISION_LITERAL_V1,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5: RED_FLAG_DETECTOR_REVISION_NEGATION_AWARE_V2,
}

# Fields that must be present for each structured kind.
_REQUIRED_FIELDS: dict[OutputKind, list[str]] = {
    "diagnosis": ["candidates"],
    "treatment": ["goal", "interventions"],
    "posture": ["view", "findings", "summary_markdown", "disclaimer"],
    "assessment": ["observations"],
}

_SAFETY_FALLBACK: dict[OutputKind, str] = {
    "diagnosis": (
        "出于安全考虑，本次未能生成可下发的诊断结论。"
        "请补充更具体的症状信息，或前往专业医疗机构进一步评估。"
        "本系统不构成医疗诊断。"
    ),
    "treatment": (
        "出于安全考虑，本次未能生成可下发的训练方案。"
        "请确认诊断结论后重试，或咨询专业康复/医疗人员获取个性化方案。"
        "本系统不构成医疗处方。"
    ),
    "posture": (
        "出于安全考虑，本次未能生成可下发的体态分析。"
        "请重新上传清晰的站姿照片，或咨询专业人士评估。"
        "本分析不构成医疗诊断。"
    ),
    "assessment": (
        "本次 Assessment 输出未通过证据一致性校验，因此未写入任何健康观察。请补充可验证资料后重试。"
    ),
}


@dataclass
class GuardedOutput:
    """Result of the forced governance gate."""

    verdict: Literal["accepted", "degraded", "rejected"]
    kind: OutputKind
    # Payload safe to emit/persist. None when rejected (raw content blocked).
    payload: dict[str, Any] | None
    reasons: list[str] = field(default_factory=list)
    issues: list[dict[str, Any]] = field(default_factory=list)
    safety_fallback: str | None = None

    def to_emit_dict(self) -> dict[str, Any]:
        """Shape returned to HTTP / runtime callers.

        Rejected responses deliberately omit the raw model payload so a
        misbehaving client cannot recover unsafe content from the body.
        """
        governance = {
            "verdict": self.verdict,
            "kind": self.kind,
            "reasons": list(self.reasons),
            "issues": list(self.issues),
        }
        if self.verdict == "rejected":
            return {
                "governance": governance,
                "safety_fallback": self.safety_fallback or _SAFETY_FALLBACK[self.kind],
            }

        body = dict(self.payload or {})
        body["governance"] = governance
        if self.verdict == "degraded":
            body.setdefault(
                "safety_note",
                "输出已通过治理但置信度降低，请结合专业意见谨慎参考。",
            )
        return body

    def to_safety_events(self) -> list[dict[str, Any]]:
        """Internal event dicts for the consultation NDJSON/SSE bridge.

        Always emit ``safety.output_reviewed``. On reject also emit
        ``safety.output_rejected`` so Go can persist both via the event log.
        """
        reviewed = {
            "type": "safety.output_reviewed",
            "kind": self.kind,
            "verdict": self.verdict,
            "reasons": list(self.reasons),
            "issues": list(self.issues),
        }
        events = [reviewed]
        if self.verdict == "rejected":
            events.append(
                {
                    "type": "safety.output_rejected",
                    "kind": self.kind,
                    "verdict": "rejected",
                    "reasons": list(self.reasons),
                    "safety_fallback": self.safety_fallback or _SAFETY_FALLBACK[self.kind],
                }
            )
        return events


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
_DIAGNOSIS_V4_CANDIDATE_EDUCATION_FIELDS = frozenset(
    {"name", "typical_symptoms", "differential"}
)

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


def _collect_issues(
    kind: OutputKind,
    payload: dict[str, Any],
    *,
    policy_revision: str | None,
    rag_results: list[dict[str, Any]] | None,
    extracted_info: list[dict[str, Any]] | None,
    assessment_evidence_catalog: dict[str, AssessmentEvidenceItem] | None,
) -> list[GovernanceIssue]:
    """Run schema + red_flag + (treatment) faithfulness policies."""
    issues: list[GovernanceIssue] = []
    issues.extend(check_schema_valid(payload, _REQUIRED_FIELDS[kind]))

    if kind == "assessment":
        issues.extend(assessment_evidence_issues(payload, assessment_evidence_catalog or {}))

    claim_text = _clinical_claim_text(payload)
    if kind == "diagnosis" and policy_revision in {
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
    }:
        claim_text = _diagnosis_v4_current_claim_text(payload)
    detector_revision = DEFAULT_RED_FLAG_DETECTOR_REVISION
    if kind == "diagnosis":
        assert policy_revision is not None
        detector_revision = DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY[policy_revision]
    issues.extend(
        check_red_flags(
            claim_text,
            {"extracted_info": []},
            detector_revision=detector_revision,
        )
    )

    if kind == "treatment" and rag_results:
        ctx = GovernanceContext(
            output_type="treatment",
            rag_results=rag_results,
            extracted_info=extracted_info or [],
        )
        issues.extend(check_faithfulness(payload, ctx))

    return issues


def _decide_verdict(kind: OutputKind, issues: list[GovernanceIssue]) -> GovernanceStatus:
    """Map issues to a verdict with kind-aware P2 policy."""
    if not issues:
        return GovernanceStatus.ACCEPTED

    hard_reject = False
    soft_degrade = False

    for issue in issues:
        policy = issue.policy
        severity = issue.severity

        if policy.startswith("schema") and severity in (
            IssueSeverity.ERROR,
            IssueSeverity.CRITICAL,
        ):
            hard_reject = True
            continue

        if policy.startswith("red_flag"):
            # Posture: red flags lower confidence but still surface findings.
            # Diagnosis/treatment: hard-block clinical red-flag claims.
            if kind == "posture":
                soft_degrade = True
            else:
                hard_reject = True
            continue

        if policy.startswith("faithfulness"):
            soft_degrade = True
            continue

        if severity in (IssueSeverity.ERROR, IssueSeverity.CRITICAL):
            hard_reject = True
        elif severity == IssueSeverity.WARNING:
            soft_degrade = True

    if hard_reject:
        return GovernanceStatus.REJECTED
    if soft_degrade:
        return GovernanceStatus.DEGRADED
    return GovernanceStatus.ACCEPTED


def guard_structured_output(
    kind: OutputKind,
    payload: dict[str, Any],
    *,
    rag_results: list[dict[str, Any]] | None = None,
    extracted_info: list[dict[str, Any]] | None = None,
    policy_revision: str | None = None,
    assessment_evidence_catalog: dict[str, AssessmentEvidenceItem] | None = None,
) -> GuardedOutput:
    """Force-gate a structured diagnosis, treatment, posture, or Assessment payload."""
    effective_policy_revision = policy_revision
    if kind == "diagnosis" and policy_revision is not None:
        if policy_revision not in {
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V3,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
            DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
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
