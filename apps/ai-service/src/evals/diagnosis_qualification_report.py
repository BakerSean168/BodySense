"""Turn qualification runs into slice-aware evidence and paired comparisons."""

from __future__ import annotations

import json
from typing import Any

from src.evals.diagnosis_qualification_dataset import load_dataset_document
from src.evals.diagnosis_qualification_schema import (
    DEFAULT_QUALIFICATION_POLICY,
    SERVICE_ROOT,
    DiagnosisEvalExecution,
    DiagnosisQualificationRun,
)


def _bucket_increment(buckets: dict[str, dict[str, int]], name: str, passed: bool) -> None:
    bucket = buckets.setdefault(name, {"passed": 0, "total": 0})
    bucket["total"] += 1
    bucket["passed"] += int(passed)


def _rate(stats: dict[str, int]) -> float:
    total = stats.get("total", 0)
    return stats.get("passed", 0) / total if total else 0.0


def report_summary(run: DiagnosisQualificationRun) -> dict[str, Any]:
    """Convert Pydantic Evals output into stable slice-aware qualification evidence."""

    cases: list[dict[str, Any]] = []
    passed = 0
    split_totals: dict[str, dict[str, int]] = {}
    slice_totals: dict[str, dict[str, int]] = {}
    evaluator_totals: dict[str, dict[str, int]] = {}
    critical_failures: list[str] = []

    for case in run.report.cases:
        assertions = {name: bool(result.value) for name, result in case.assertions.items()}
        case_passed = bool(assertions) and all(assertions.values()) and not case.evaluator_failures
        passed += int(case_passed)
        metadata = case.metadata
        split = str(getattr(metadata, "split", "unknown"))
        slices = list(getattr(metadata, "slices", ["unknown"]))
        critical = bool(getattr(metadata, "critical", False))
        _bucket_increment(split_totals, split, case_passed)
        for slice_name in slices:
            _bucket_increment(slice_totals, str(slice_name), case_passed)
        for evaluator_name, assertion_passed in assertions.items():
            _bucket_increment(evaluator_totals, evaluator_name, assertion_passed)
        if critical and not case_passed:
            critical_failures.append(str(case.name))

        execution = case.output
        trace = execution.trace if isinstance(execution, DiagnosisEvalExecution) else None
        cases.append(
            {
                "name": case.name,
                "passed": case_passed,
                "split": split,
                "slices": slices,
                "critical": critical,
                "applicability_reason": getattr(metadata, "applicability_reason", None),
                "assertions": assertions,
                "trace": trace.model_dump(mode="json") if trace is not None else {},
            }
        )

    total = len(run.report.cases)
    overall_pass_rate = passed / total if total else 0.0
    required_splits = set(DEFAULT_QUALIFICATION_POLICY["required_splits"])
    missing_splits = sorted(required_splits - set(split_totals))
    critical_slices = list(DEFAULT_QUALIFICATION_POLICY["critical_slices"])
    critical_slice_failures = [
        name
        for name in critical_slices
        if name not in slice_totals
        or _rate(slice_totals[name])
        < float(DEFAULT_QUALIFICATION_POLICY["minimum_critical_slice_pass_rate"])
    ]
    reasons: list[str] = []
    if missing_splits:
        reasons.append(f"missing required dataset splits: {', '.join(missing_splits)}")
    if overall_pass_rate < float(DEFAULT_QUALIFICATION_POLICY["minimum_overall_pass_rate"]):
        reasons.append(f"overall pass rate below qualification threshold: {overall_pass_rate:.3f}")
    if critical_failures:
        reasons.append(f"critical case failures: {', '.join(critical_failures)}")
    if critical_slice_failures:
        reasons.append(f"critical slice gate failed: {', '.join(critical_slice_failures)}")

    return {
        "name": run.report.name,
        "mode": run.mode,
        "dataset": {
            "path": str(run.dataset_path.relative_to(SERVICE_ROOT)),
            "fingerprint": run.dataset_fingerprint,
            "full_case_count": len(load_dataset_document(run.dataset_path).cases),
        },
        "configuration_id": run.configuration.configuration_id,
        "configuration": run.configuration.provenance(),
        "passed": passed,
        "total": total,
        "failed": total - passed,
        "pass_rate": overall_pass_rate,
        "splits": split_totals,
        "slices": slice_totals,
        "evaluators": evaluator_totals,
        "cases": cases,
        "qualification": {
            "qualified": not reasons,
            "policy": DEFAULT_QUALIFICATION_POLICY,
            "critical_failures": critical_failures,
            "reasons": reasons,
        },
    }


def compare_qualification_summaries(
    champion: dict[str, Any],
    candidate: dict[str, Any],
    *,
    margin: float | None = None,
) -> dict[str, Any]:
    """Paired non-inferiority comparison on the same immutable qualification dataset."""

    if champion.get("dataset", {}).get("fingerprint") != candidate.get("dataset", {}).get(
        "fingerprint"
    ):
        raise ValueError("qualification comparison requires the same dataset fingerprint")

    champion_cases = {str(case["name"]): case for case in champion.get("cases", [])}
    candidate_cases = {str(case["name"]): case for case in candidate.get("cases", [])}
    champion_only = sorted(champion_cases.keys() - candidate_cases.keys())
    if champion_only:
        raise ValueError(f"candidate report is missing Champion cases: {', '.join(champion_only)}")
    shared_names = sorted(champion_cases)
    candidate_only = sorted(candidate_cases.keys() - champion_cases.keys())
    for name in shared_names:
        for field in ("split", "slices", "critical"):
            if champion_cases[name].get(field) != candidate_cases[name].get(field):
                raise ValueError(f"paired case taxonomy differs for {name}: {field}")

    effective_margin = (
        float(DEFAULT_QUALIFICATION_POLICY["non_inferiority_margin"]) if margin is None else margin
    )
    regressions = sorted(
        name
        for name, baseline_case in champion_cases.items()
        if baseline_case.get("passed") and not candidate_cases[name].get("passed")
    )
    improvements = sorted(
        name
        for name, baseline_case in champion_cases.items()
        if not baseline_case.get("passed") and candidate_cases[name].get("passed")
    )
    critical_regressions = [name for name in regressions if champion_cases[name].get("critical")]

    champion_rate = sum(bool(case.get("passed")) for case in champion_cases.values()) / max(
        1, len(champion_cases)
    )
    candidate_rate = sum(bool(candidate_cases[name].get("passed")) for name in shared_names) / max(
        1, len(shared_names)
    )
    pass_rate_delta = candidate_rate - champion_rate

    all_slices = {
        slice_name for name in shared_names for slice_name in champion_cases[name].get("slices", [])
    }
    slice_deltas: dict[str, float] = {}
    slice_regressions: list[str] = []
    for name in sorted(all_slices):
        champion_stats = {
            "passed": sum(
                bool(champion_cases[case_name].get("passed"))
                for case_name in shared_names
                if name in champion_cases[case_name].get("slices", [])
            ),
            "total": sum(
                name in champion_cases[case_name].get("slices", []) for case_name in shared_names
            ),
        }
        candidate_stats = {
            "passed": sum(
                bool(candidate_cases[case_name].get("passed"))
                for case_name in shared_names
                if name in candidate_cases[case_name].get("slices", [])
            ),
            "total": sum(
                name in candidate_cases[case_name].get("slices", []) for case_name in shared_names
            ),
        }
        delta = _rate(candidate_stats) - _rate(champion_stats)
        slice_deltas[name] = delta
        if delta < -effective_margin:
            slice_regressions.append(name)

    non_inferior = (
        pass_rate_delta >= -effective_margin and not critical_regressions and not slice_regressions
    )
    return {
        "champion_configuration_id": champion.get("configuration_id"),
        "candidate_configuration_id": candidate.get("configuration_id"),
        "margin": effective_margin,
        "shared_case_count": len(shared_names),
        "shared_case_names": shared_names,
        "candidate_only_case_names": candidate_only,
        "candidate_only_cases": [
            {
                "name": name,
                "passed": bool(candidate_cases[name].get("passed")),
                "applicability_reason": candidate_cases[name].get("applicability_reason"),
            }
            for name in candidate_only
        ],
        "champion_pass_rate": champion_rate,
        "candidate_pass_rate": candidate_rate,
        "pass_rate_delta": pass_rate_delta,
        "regressions": regressions,
        "improvements": improvements,
        "critical_regressions": critical_regressions,
        "slice_deltas": slice_deltas,
        "slice_regressions": slice_regressions,
        "non_inferior": non_inferior,
        "promotion_eligible": bool(champion.get("qualification", {}).get("qualified"))
        and bool(candidate.get("qualification", {}).get("qualified"))
        and non_inferior,
    }


def render_summary(
    run: DiagnosisQualificationRun,
    *,
    comparison: dict[str, Any] | None = None,
) -> str:
    """Render concise human-readable qualification evidence."""

    summary = report_summary(run)
    qualification = summary["qualification"]
    lines = [
        "# Diagnosis Pydantic Evals Qualification",
        "",
        f"- Result: {summary['passed']}/{summary['total']} passed",
        f"- Qualified: {'YES' if qualification['qualified'] else 'NO'}",
        f"- Agent configuration: `{summary['configuration_id']}`",
        f"- Dataset fingerprint: `{summary['dataset']['fingerprint']}`",
        f"- Mode: {summary['mode']}",
        "",
        "## Splits",
    ]
    for name, stats in sorted(summary["splits"].items()):
        lines.append(f"- `{name}`: {stats['passed']}/{stats['total']} passed")
    lines.extend(["", "## Risk slices"])
    for name, stats in sorted(summary["slices"].items()):
        lines.append(f"- `{name}`: {stats['passed']}/{stats['total']} passed")
    lines.extend(["", "## Cases"])
    for case in summary["cases"]:
        mark = "PASS" if case["passed"] else "FAIL"
        lines.append(f"- `{mark}` `{case['name']}` [{case['split']}] ({', '.join(case['slices'])})")
    if qualification["reasons"]:
        lines.extend(["", "## Qualification failures"])
        lines.extend(f"- {reason}" for reason in qualification["reasons"])
    if comparison is not None:
        lines.extend(
            [
                "",
                "## Champion comparison",
                f"- Non-inferior: {'YES' if comparison['non_inferior'] else 'NO'}",
                f"- Promotion eligible: {'YES' if comparison['promotion_eligible'] else 'NO'}",
                f"- Pass-rate delta: {comparison['pass_rate_delta']:+.3f}",
                f"- Shared paired cases: {comparison['shared_case_count']}",
                "- Candidate-only cases: "
                + (", ".join(comparison["candidate_only_case_names"]) or "none"),
            ]
        )
    return "\n".join(lines) + "\n"


def summary_json(
    run: DiagnosisQualificationRun,
    *,
    comparison: dict[str, Any] | None = None,
) -> str:
    summary = report_summary(run)
    if comparison is not None:
        summary["comparison"] = comparison
    return json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
