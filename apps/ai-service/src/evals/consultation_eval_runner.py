"""Structured consultation eval runner for agent quality checks."""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import yaml

from src.models.consultation import ChatContext, ExtractedInfo
from src.services.agent_workflow import ConsultationAgentWorkflow
from src.services.faithfulness_checker import get_faithfulness_checker
from src.services.red_flag_detector import get_red_flag_detector

SERVICE_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CASES_PATH = SERVICE_ROOT / "data" / "evals" / "consultation_eval_cases.yaml"
DEFAULT_OUTPUT_DIR = SERVICE_ROOT / "data" / "benchmarks" / "consultation-evals"


@dataclass
class EvalCaseResult:
    """Result for one eval case."""

    suite: str
    case_id: str
    title: str
    passed: bool
    details: dict[str, Any]

    def to_dict(self) -> dict[str, Any]:
        """Serialize for report output."""
        return {
            "suite": self.suite,
            "case_id": self.case_id,
            "title": self.title,
            "passed": self.passed,
            "details": self.details,
        }


def load_eval_cases(cases_path: Path) -> dict[str, list[dict[str, Any]]]:
    """Load eval cases from YAML."""
    with cases_path.open("r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle) or {}
    return {suite: cases for suite, cases in data.items() if isinstance(cases, list)}


def _build_context(raw_context: dict[str, Any] | None) -> ChatContext:
    """Build a lightweight chat context for workflow evaluation."""
    context = raw_context or {}
    return ChatContext(
        session_id="eval-session",
        user_id="eval-user",
        phase=context.get("phase", "collecting"),
        extracted_info=ExtractedInfo.from_list(context.get("extracted_symptoms", [])),
        messages=context.get("messages", []),
        profile=context.get("profile", {}),
    )


def _round_rate(passed: int, total: int) -> float:
    """Return pass rate percentage rounded for reports."""
    if total == 0:
        return 0.0
    return round((passed / total) * 100, 2)


def _run_workflow_suite(cases: list[dict[str, Any]]) -> list[EvalCaseResult]:
    """Run workflow intent and routing evals."""
    workflow = ConsultationAgentWorkflow()
    results: list[EvalCaseResult] = []

    for case in cases:
        expected = case.get("expect", {})
        context = _build_context(case.get("context"))
        message = case.get("message", "")

        intent = workflow.classify_intent(message, context)
        decision = workflow.decide_next_action(intent, context, message)
        should_interrupt = workflow.should_interrupt_for_follow_up(message, context)
        should_analyze = workflow.should_analyze(context.extracted_info)

        checks = {
            "intent": expected.get("intent") == intent.value
            if "intent" in expected
            else True,
            "action": expected.get("action") == decision.action.value
            if "action" in expected
            else True,
            "should_interrupt_for_follow_up": (
                expected.get("should_interrupt_for_follow_up") == should_interrupt
                if "should_interrupt_for_follow_up" in expected
                else True
            ),
            "should_analyze": expected.get("should_analyze") == should_analyze
            if "should_analyze" in expected
            else True,
        }
        passed = all(checks.values())

        results.append(
            EvalCaseResult(
                suite="workflow",
                case_id=case["id"],
                title=case["title"],
                passed=passed,
                details={
                    "checks": checks,
                    "expected": expected,
                    "actual": {
                        "intent": intent.value,
                        "action": decision.action.value,
                        "should_interrupt_for_follow_up": should_interrupt,
                        "should_analyze": should_analyze,
                        "reasoning": decision.reasoning,
                    },
                },
            )
        )

    return results


def _run_red_flag_suite(cases: list[dict[str, Any]]) -> list[EvalCaseResult]:
    """Run safety recall evals for red flag detection."""
    detector = get_red_flag_detector()
    results: list[EvalCaseResult] = []

    for case in cases:
        expected = case.get("expect", {})
        result = detector.detect(
            extracted_info=case.get("extracted_info", []),
            conversation_text=case.get("conversation_text", ""),
        )
        actual_categories = [flag.category for flag in result.flags]
        expected_categories = expected.get("categories", [])
        category_check = all(category in actual_categories for category in expected_categories)
        if expected.get("has_red_flags") is False:
            category_check = actual_categories == expected_categories

        checks = {
            "has_red_flags": result.has_red_flags == expected.get("has_red_flags"),
            "categories": category_check,
        }
        passed = all(checks.values())

        results.append(
            EvalCaseResult(
                suite="red_flags",
                case_id=case["id"],
                title=case["title"],
                passed=passed,
                details={
                    "checks": checks,
                    "expected": expected,
                    "actual": {
                        "has_red_flags": result.has_red_flags,
                        "categories": actual_categories,
                    },
                },
            )
        )

    return results


def _run_faithfulness_suite(cases: list[dict[str, Any]]) -> list[EvalCaseResult]:
    """Run groundedness evals for treatment generation."""
    checker = get_faithfulness_checker()
    results: list[EvalCaseResult] = []

    for case in cases:
        expected = case.get("expect", {})
        result = checker.check_treatment_faithfulness(
            treatment_plan=case.get("treatment_plan", {}),
            rag_results=case.get("rag_results", []),
        )
        grounded = [exercise.name for exercise in result.exercises if exercise.grounded]

        checks = {
            "faithful": result.faithful == expected.get("faithful"),
            "grounded_exercises": all(
                name in grounded for name in expected.get("grounded_exercises", [])
            ),
            "ungrounded_exercises": all(
                name in result.ungrounded_exercises
                for name in expected.get("ungrounded_exercises", [])
            ),
        }
        passed = all(checks.values())

        results.append(
            EvalCaseResult(
                suite="faithfulness",
                case_id=case["id"],
                title=case["title"],
                passed=passed,
                details={
                    "checks": checks,
                    "expected": expected,
                    "actual": {
                        "faithful": result.faithful,
                        "grounded_exercises": grounded,
                        "ungrounded_exercises": result.ungrounded_exercises,
                    },
                },
            )
        )

    return results


def run_consultation_evals(cases_path: Path = DEFAULT_CASES_PATH) -> dict[str, Any]:
    """Run all consultation eval suites and return a structured report."""
    cases_by_suite = load_eval_cases(cases_path)

    case_results: list[EvalCaseResult] = []
    case_results.extend(_run_workflow_suite(cases_by_suite.get("workflow", [])))
    case_results.extend(_run_red_flag_suite(cases_by_suite.get("red_flags", [])))
    case_results.extend(_run_faithfulness_suite(cases_by_suite.get("faithfulness", [])))

    suite_summaries: list[dict[str, Any]] = []
    for suite_name in ("workflow", "red_flags", "faithfulness"):
        suite_cases = [case for case in case_results if case.suite == suite_name]
        passed_cases = sum(1 for case in suite_cases if case.passed)
        suite_summaries.append(
            {
                "suite": suite_name,
                "total_cases": len(suite_cases),
                "passed_cases": passed_cases,
                "failed_cases": len(suite_cases) - passed_cases,
                "pass_rate": _round_rate(passed_cases, len(suite_cases)),
            }
        )

    total_cases = len(case_results)
    passed_cases = sum(1 for case in case_results if case.passed)
    failed_cases = total_cases - passed_cases

    return {
        "generated_at": datetime.now(UTC).isoformat(),
        "cases_path": str(cases_path),
        "overall": {
            "total_cases": total_cases,
            "passed_cases": passed_cases,
            "failed_cases": failed_cases,
            "pass_rate": _round_rate(passed_cases, total_cases),
        },
        "suites": suite_summaries,
        "cases": [case.to_dict() for case in case_results],
    }


def render_markdown_summary(report: dict[str, Any]) -> str:
    """Render a concise markdown summary for humans."""
    overall = report["overall"]
    lines = [
        "# Consultation Eval Summary",
        "",
        f"- Generated at: `{report['generated_at']}`",
        f"- Cases file: `{report['cases_path']}`",
        (
            f"- Overall: `{overall['passed_cases']}/{overall['total_cases']}` passed"
            f" (`{overall['pass_rate']}%`)"
        ),
        "",
        "## Suite Overview",
        "",
        "| Suite | Passed | Total | Pass Rate |",
        "| --- | ---: | ---: | ---: |",
    ]

    for suite in report["suites"]:
        lines.append(
            "| {suite} | {passed_cases} | {total_cases} | {pass_rate}% |".format(**suite)
        )

    failed_cases = [case for case in report["cases"] if not case["passed"]]
    lines.extend(["", "## Failed Cases", ""])
    if not failed_cases:
        lines.append("- None.")
    else:
        for case in failed_cases:
            lines.append(f"- `{case['suite']}/{case['case_id']}`: {case['title']}")

    return "\n".join(lines) + "\n"


def write_report_files(
    report: dict[str, Any],
    output_dir: Path = DEFAULT_OUTPUT_DIR,
) -> dict[str, Path]:
    """Persist report files under the benchmark output directory."""
    output_dir.mkdir(parents=True, exist_ok=True)

    json_path = output_dir / "consultation-eval-summary.json"
    markdown_path = output_dir / "consultation-eval-summary.md"

    json_path.write_text(
        json.dumps(report, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    markdown_path.write_text(render_markdown_summary(report), encoding="utf-8")

    return {
        "json": json_path,
        "markdown": markdown_path,
    }


def main(argv: list[str] | None = None) -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--cases",
        type=Path,
        default=DEFAULT_CASES_PATH,
        help="Path to the consultation eval cases YAML file.",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=DEFAULT_OUTPUT_DIR,
        help="Directory for generated eval reports.",
    )
    parser.add_argument(
        "--stdout-only",
        action="store_true",
        help="Print the report but skip writing files.",
    )
    args = parser.parse_args(argv)

    report = run_consultation_evals(args.cases)
    print(render_markdown_summary(report))

    if not args.stdout_only:
        outputs = write_report_files(report, args.output_dir)
        print(f"Wrote JSON report to {outputs['json']}")
        print(f"Wrote Markdown report to {outputs['markdown']}")

    return 0 if report["overall"]["failed_cases"] == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
