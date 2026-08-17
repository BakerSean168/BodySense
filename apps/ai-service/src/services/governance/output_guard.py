"""AIOutputGuard — centralizes validation of AI outputs against governance policies."""

from __future__ import annotations

import logging
from typing import Any

from .policies import check_empty_output, check_red_flags, check_schema_valid
from .types import GovernanceIssue, GovernanceResult, GovernanceStatus

logger = logging.getLogger(__name__)


class AIOutputGuard:
    """Validates AI outputs against a set of governance policies.

    Policies are run in order. The final status is determined by the
    most severe issue found:
    - No issues → accepted
    - Warning issues → degraded
    - Error issues → rejected
    - Critical issues → rejected
    """

    def validate_text_output(
        self,
        output_text: str,
        context: dict[str, Any] | None = None,
    ) -> GovernanceResult:
        """Validate a text output (e.g., diagnosis, treatment plan)."""
        ctx = context or {}
        issues: list[GovernanceIssue] = []

        # Run policies
        issues.extend(check_empty_output(output_text))
        issues.extend(check_red_flags(output_text, ctx))

        return self._build_result(issues, validated_output=output_text)

    def validate_structured_output(
        self,
        output: dict[str, Any],
        required_fields: list[str] | None = None,
        context: dict[str, Any] | None = None,
    ) -> GovernanceResult:
        """Validate a structured output (e.g., diagnosis JSON)."""
        ctx = context or {}
        issues: list[GovernanceIssue] = []

        # Schema validation
        if required_fields:
            issues.extend(check_schema_valid(output, required_fields))

        # Red flag check on serialized output
        import json

        output_text = json.dumps(output, ensure_ascii=False)
        issues.extend(check_red_flags(output_text, ctx))

        return self._build_result(issues, validated_output=output)

    def _build_result(
        self,
        issues: list[GovernanceIssue],
        validated_output: Any = None,
    ) -> GovernanceResult:
        """Determine overall status from issues."""
        if not issues:
            return GovernanceResult(
                status=GovernanceStatus.ACCEPTED,
                validated_output=validated_output,
            )

        max_severity = max(
            (i.severity.value for i in issues),
            key=lambda s: {"info": 0, "warning": 1, "error": 2, "critical": 3}.get(s, 0),
        )

        if max_severity in ("error", "critical"):
            status = GovernanceStatus.REJECTED
        elif max_severity == "warning":
            status = GovernanceStatus.DEGRADED
        else:
            status = GovernanceStatus.ACCEPTED

        return GovernanceResult(
            status=status,
            issues=issues,
            validated_output=validated_output,
        )
