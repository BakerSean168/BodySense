"""Governance policies — individual validation checks for AI outputs."""

from __future__ import annotations

from typing import Any

from ..red_flag_detector import get_red_flag_detector
from .types import GovernanceIssue, IssueSeverity


def check_red_flags(output_text: str, context: dict[str, Any]) -> list[GovernanceIssue]:
    """Check for red flags in the output text."""
    issues: list[GovernanceIssue] = []
    detector = get_red_flag_detector()

    # Scan the output text for red flag patterns
    extracted_info = context.get("extracted_info", [])
    result = detector.detect(extracted_info, output_text)

    if result.has_red_flags:
        for flag in result.flags:
            issues.append(GovernanceIssue(
                policy="red_flag_safety",
                severity=IssueSeverity.WARNING,
                message=flag.message,
                details={"category": flag.category, "matched_text": flag.matched_text},
            ))

    return issues


def check_schema_valid(output: dict[str, Any], required_fields: list[str]) -> list[GovernanceIssue]:
    """Check that the output contains required fields."""
    issues: list[GovernanceIssue] = []
    for field in required_fields:
        if field not in output or output[field] is None:
            issues.append(GovernanceIssue(
                policy="schema_validation",
                severity=IssueSeverity.ERROR,
                message=f"Missing required field: {field}",
            ))
    return issues


def check_empty_output(output_text: str) -> list[GovernanceIssue]:
    """Check for empty or trivially short outputs."""
    if not output_text or len(output_text.strip()) < 10:
        return [GovernanceIssue(
            policy="completeness",
            severity=IssueSeverity.WARNING,
            message="Output is empty or too short",
        )]
    return []
