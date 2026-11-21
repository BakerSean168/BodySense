"""Result type returned by the forced governance gate."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal

from .governance_policy import _SAFETY_FALLBACK, OutputKind


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
