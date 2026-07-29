"""Single governance seam for structured AI outputs on the live path.

Diagnosis, treatment, and posture leave Python only after passing through
``guard_structured_output``. Callers must not invent parallel policy ifs —
this module is the forced gate before emit/persist.

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

from ..services.governance.output_guard import AIOutputGuard
from ..services.governance.policies import check_faithfulness, check_red_flags, check_schema_valid
from ..services.governance.types import (
    GovernanceContext,
    GovernanceIssue,
    GovernanceStatus,
    IssueSeverity,
)

logger = logging.getLogger(__name__)

OutputKind = Literal["diagnosis", "treatment", "posture"]

# Fields that must be present for each structured kind.
_REQUIRED_FIELDS: dict[OutputKind, list[str]] = {
    "diagnosis": ["diagnoses"],
    "treatment": ["treatment_plan"],
    "posture": ["view", "findings", "summary_markdown", "disclaimer"],
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
                    "safety_fallback": self.safety_fallback
                    or _SAFETY_FALLBACK[self.kind],
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
        "safety_fallback",
    }
)


def _clinical_claim_text(payload: dict[str, Any]) -> str:
    """Serialize only the clinical claim surface for red-flag scanning."""

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


def _collect_issues(
    kind: OutputKind,
    payload: dict[str, Any],
    *,
    rag_results: list[dict[str, Any]] | None,
    extracted_info: list[dict[str, Any]] | None,
) -> list[GovernanceIssue]:
    """Run schema + red_flag + (treatment) faithfulness policies."""
    issues: list[GovernanceIssue] = []
    required = _REQUIRED_FIELDS[kind]
    issues.extend(check_schema_valid(payload, required))

    if kind == "treatment":
        plan = payload.get("treatment_plan")
        if isinstance(plan, dict):
            issues.extend(check_schema_valid(plan, ["correction_exercises"]))

    issues.extend(
        check_red_flags(
            _clinical_claim_text(payload),
            {"extracted_info": []},
        )
    )

    if kind == "treatment" and rag_results:
        plan = payload.get("treatment_plan")
        if isinstance(plan, dict):
            ctx = GovernanceContext(
                output_type="treatment",
                rag_results=rag_results,
                extracted_info=extracted_info or [],
            )
            issues.extend(check_faithfulness(plan, ctx))

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
) -> GuardedOutput:
    """Force-gate a structured diagnosis, treatment, or posture payload."""
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
        rag_results=rag_results,
        extracted_info=extracted_info,
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
