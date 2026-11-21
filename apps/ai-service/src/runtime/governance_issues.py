"""Issue collection and verdict mapping for the governance seam."""

from __future__ import annotations

from typing import Any

from pydantic import ValidationError

from ..models.diagnosis import DiagnosisSafetyFindingV1
from ..models.safety import SafetyEnvelopeV2
from ..services.assessment_evidence import (
    AssessmentEvidenceItem,
    assessment_evidence_issues,
)
from ..services.governance.policies import check_faithfulness, check_red_flags, check_schema_valid
from ..services.governance.types import (
    GovernanceContext,
    GovernanceIssue,
    GovernanceStatus,
    IssueSeverity,
)
from ..services.red_flag_detector import DEFAULT_RED_FLAG_DETECTOR_REVISION
from .governance_policy import (
    _REQUIRED_FIELDS,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
    DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8,
    DIAGNOSIS_RED_FLAG_DETECTOR_REVISION_BY_POLICY,
    OutputKind,
)
from .governance_scan import (
    _clinical_claim_text,
    _diagnosis_v4_current_claim_text,
)


def _collect_issues(
    kind: OutputKind,
    payload: dict[str, Any],
    *,
    policy_revision: str | None,
    rag_results: list[dict[str, Any]] | None,
    extracted_info: list[dict[str, Any]] | None,
    assessment_evidence_catalog: dict[str, AssessmentEvidenceItem] | None,
    body_state: dict[str, Any] | None,
    safety_envelope: SafetyEnvelopeV2 | None,
) -> list[GovernanceIssue]:
    """Run schema + red_flag + (treatment) faithfulness policies."""
    issues: list[GovernanceIssue] = []
    issues.extend(check_schema_valid(payload, _REQUIRED_FIELDS[kind]))

    if kind == "diagnosis" and policy_revision == DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V8:
        if safety_envelope is not None and any(
            blocker.risk_level == "urgent" for blocker in safety_envelope.active_blockers
        ):
            issues.append(
                GovernanceIssue(
                    "schema.structured_safety",
                    IssueSeverity.ERROR,
                    "active_structured_safety_blocker",
                )
            )
        if body_state is None:
            issues.append(
                GovernanceIssue(
                    "schema.structured_safety",
                    IssueSeverity.ERROR,
                    "pinned BodyState required for safety findings",
                )
            )
        else:
            refs = {
                f"body-state:{kind}:{item['id']}"
                for kind, key in (("fact", "facts"), ("observation", "observations"))
                for item in body_state.get(key, [])
                if isinstance(item, dict) and isinstance(item.get("id"), str)
            }
            findings = payload.get("safety_findings", [])
            if not isinstance(findings, list):
                issues.append(
                    GovernanceIssue(
                        "schema.structured_safety",
                        IssueSeverity.ERROR,
                        "safety_findings must be a list",
                    )
                )
            else:
                for raw in findings:
                    try:
                        finding = DiagnosisSafetyFindingV1.model_validate(raw)
                        if any(ref not in refs for ref in finding.source_refs):
                            raise ValueError("unresolvable safety finding source ref")
                    except (ValidationError, ValueError):
                        issues.append(
                            GovernanceIssue(
                                "schema.structured_safety",
                                IssueSeverity.ERROR,
                                "invalid or unresolvable safety finding",
                            )
                        )
        return issues

    if kind == "assessment":
        issues.extend(assessment_evidence_issues(payload, assessment_evidence_catalog or {}))

    claim_text = _clinical_claim_text(payload)
    if kind == "diagnosis" and policy_revision in {
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V4,
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V5,
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V6,
        DIAGNOSIS_GOVERNANCE_POLICY_REVISION_V7,
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
