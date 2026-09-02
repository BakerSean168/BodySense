"""Evaluate one privacy-bounded health-document benchmark summary against frozen gates."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

SERVICE_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_THRESHOLDS = SERVICE_ROOT / "benchmarks" / "health_document" / "thresholds-v2.json"


def _require_max(
    reasons: list[str],
    *,
    label: str,
    actual: float | int | None,
    maximum: float | int,
) -> None:
    if actual is None:
        reasons.append(f"{label}:missing")
    elif actual > maximum:
        reasons.append(f"{label}:{actual}>{maximum}")


def _require_min(
    reasons: list[str],
    *,
    label: str,
    actual: float | int | None,
    minimum: float | int,
) -> None:
    if actual is None:
        reasons.append(f"{label}:missing")
    elif actual < minimum:
        reasons.append(f"{label}:{actual}<{minimum}")


def evaluate(summary: dict[str, Any], thresholds: dict[str, Any]) -> dict[str, Any]:
    reasons: list[str] = []
    runtime = summary.get("runtime") or {}
    fixture_execution = summary.get("fixture_execution") or {}
    selection_scope = thresholds.get("selection_scope")

    if selection_scope == "verified-evidence-authority":
        authority = summary.get("evidence_authority")
        if not isinstance(authority, dict):
            reasons.append("evidence_authority:missing")
            authority = {}
        authority_gates = thresholds["evidence_authority"]
        _require_max(
            reasons,
            label="authority.critical_false_admission_rate",
            actual=authority.get("critical_false_admission_rate"),
            maximum=authority_gates["critical_false_admission_rate_max"],
        )
        _require_min(
            reasons,
            label="authority.auto_admission_exact_rate",
            actual=authority.get("auto_admission_exact_rate"),
            minimum=authority_gates["auto_admission_exact_rate_min"],
        )
        _require_min(
            reasons,
            label="authority.auto_admission_coverage",
            actual=authority.get("auto_admission_coverage"),
            minimum=authority_gates["auto_admission_coverage_min"],
        )
    else:
        source = summary.get("source_accuracy")
        if not isinstance(source, dict):
            reasons.append("source_accuracy:missing")
            source = {}
        source_gates = thresholds["source_accuracy"]
        _require_max(
            reasons,
            label="source.critical_numeric_error_rate",
            actual=source.get("critical_numeric_error_rate"),
            maximum=source_gates["critical_numeric_error_rate_max"],
        )
        _require_max(
            reasons,
            label="source.safety_critical_numeric_error_rate",
            actual=source.get("critical_numeric_error_rate"),
            maximum=source_gates["safety_critical_numeric_error_rate_max"],
        )
        _require_min(
            reasons,
            label="source.name_recall",
            actual=source.get("name_recall"),
            minimum=source_gates["name_recall_min"],
        )
        _require_min(
            reasons,
            label="source.numeric_exact_match",
            actual=source.get("numeric_exact_match"),
            minimum=source_gates["numeric_exact_match_min"],
        )
        _require_min(
            reasons,
            label="source.unit_exact_match",
            actual=source.get("unit_exact_match"),
            minimum=source_gates["unit_exact_match_min"],
        )
        _require_min(
            reasons,
            label="source.row_bundle_exact_match",
            actual=source.get("row_bundle_exact_match"),
            minimum=source_gates["row_bundle_exact_match_min"],
        )

    resource_gates = thresholds["resource"]
    _require_max(
        reasons,
        label="resource.failed_fixture_count",
        actual=fixture_execution.get("failed"),
        maximum=resource_gates["failed_fixture_count_max"],
    )
    _require_max(
        reasons,
        label="resource.memory_events_max",
        actual=runtime.get("cgroup_memory_events_max"),
        maximum=resource_gates["cgroup_memory_events_max_max"],
    )
    _require_max(
        reasons,
        label="resource.memory_events_oom",
        actual=runtime.get("cgroup_memory_events_oom"),
        maximum=resource_gates["cgroup_memory_events_oom_max"],
    )
    _require_max(
        reasons,
        label="resource.memory_events_oom_kill",
        actual=runtime.get("cgroup_memory_events_oom_kill"),
        maximum=resource_gates["cgroup_memory_events_oom_kill_max"],
    )
    _require_max(
        reasons,
        label="resource.swap_peak_mb",
        actual=runtime.get("cgroup_swap_peak_mb"),
        maximum=resource_gates["cgroup_swap_peak_mb_max"],
    )

    mechanics_qualified = not reasons
    selection_blockers: list[str] = []
    if not mechanics_qualified:
        selection_blockers.append("mechanics_or_resource_gates_failed")
    if not summary.get("champion_selection_ready", False):
        selection_blockers.append(
            summary.get(
                "champion_selection_blocker",
                "real_layout_selection_evidence_missing",
            )
        )

    return {
        "schema_version": "health-document-candidate-qualification-v1",
        "thresholds_revision": thresholds["schema_version"],
        "selection_scope": selection_scope,
        "candidate_id": summary.get("candidate_id"),
        "configuration_id": summary.get("configuration_id"),
        "corpus_manifest_sha256": summary.get("corpus_manifest_sha256"),
        "harness_revision": summary.get("harness_revision"),
        "execution_topology_revision": summary.get("execution_topology_revision"),
        "mechanics_qualified": mechanics_qualified,
        "mechanics_blocking_reasons": reasons,
        "champion_selection_ready": mechanics_qualified and not selection_blockers,
        "champion_selection_blockers": selection_blockers,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--summary", type=Path, required=True)
    parser.add_argument("--thresholds", type=Path, default=DEFAULT_THRESHOLDS)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--fail-on-mechanics", action="store_true")
    args = parser.parse_args()

    summary = json.loads(args.summary.read_text(encoding="utf-8"))
    thresholds = json.loads(args.thresholds.read_text(encoding="utf-8"))
    result = evaluate(summary, thresholds)
    rendered = json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    if args.fail_on_mechanics and not result["mechanics_qualified"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
