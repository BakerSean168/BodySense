"""Governance policies — individual validation checks for AI outputs."""

from __future__ import annotations

from typing import Any

from ..faithfulness_checker import get_faithfulness_checker
from ..red_flag_detector import get_red_flag_detector
from .types import GovernanceContext, GovernanceIssue, IssueSeverity


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


def check_faithfulness(
    treatment_plan: dict[str, Any],
    context: GovernanceContext,
) -> list[GovernanceIssue]:
    """Check treatment plan faithfulness against RAG results.

    Wraps FaithfulnessChecker as a governance policy. Ungrounded exercises
    produce warning issues; all exercises ungrounded produces an error.
    """
    issues: list[GovernanceIssue] = []
    rag_results = context.rag_results

    if not rag_results:
        # No RAG results to check against — skip faithfulness check
        return issues

    checker = get_faithfulness_checker()
    result = checker.check_treatment_faithfulness(treatment_plan, rag_results)

    if not result.faithful:
        for exercise_name in result.ungrounded_exercises:
            issues.append(GovernanceIssue(
                policy="faithfulness",
                severity=IssueSeverity.WARNING,
                message=f"Exercise not grounded in knowledge base: {exercise_name}",
                details={"exercise": exercise_name},
            ))

        # If ALL exercises are ungrounded, escalate to error
        exercises = treatment_plan.get("correction_exercises", [])
        if exercises and len(result.ungrounded_exercises) == len(exercises):
            issues.append(GovernanceIssue(
                policy="faithfulness",
                severity=IssueSeverity.ERROR,
                message="All exercises are ungrounded — treatment plan may be hallucinated",
                details={"ungrounded_count": len(result.ungrounded_exercises)},
            ))

    return issues
