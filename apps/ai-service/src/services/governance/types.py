"""Types for AI output governance."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class GovernanceStatus(str, Enum):
    """Outcome of governance validation."""

    ACCEPTED = "accepted"
    REPAIRED = "repaired"
    DEGRADED = "degraded"
    REJECTED = "rejected"


class IssueSeverity(str, Enum):
    """Severity of a governance issue."""

    INFO = "info"
    WARNING = "warning"
    ERROR = "error"
    CRITICAL = "critical"


@dataclass
class GovernanceContext:
    """Context provided to governance policies for validation."""

    output_type: str = ""
    extracted_info: list[dict[str, Any]] = field(default_factory=list)
    rag_results: list[dict[str, Any]] = field(default_factory=list)
    profile: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass
class GovernanceIssue:
    """A single governance issue found during validation."""

    policy: str  # which policy found this
    severity: IssueSeverity
    message: str
    details: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "policy": self.policy,
            "severity": self.severity.value,
            "message": self.message,
            "details": self.details,
        }


@dataclass
class GovernanceResult:
    """Result of running all governance policies on an output."""

    status: GovernanceStatus
    issues: list[GovernanceIssue] = field(default_factory=list)
    validated_output: Any = None
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "status": self.status.value,
            "issues": [i.to_dict() for i in self.issues],
            "metadata": self.metadata,
        }
