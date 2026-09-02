"""Compare two immutable health-document benchmark run artifacts."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

END_TO_END_METRICS = (
    "numeric_exact_match",
    "unit_exact_match",
    "reference_range_exact_match",
    "row_association_accuracy",
    "indicator_exact_match",
    "critical_numeric_error_rate",
)

SOURCE_METRICS = (
    "name_recall",
    "numeric_exact_match",
    "unit_exact_match",
    "reference_range_exact_match",
    "row_bundle_exact_match",
    "critical_numeric_error_rate",
)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("baseline", type=Path)
    parser.add_argument("challenger", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    baseline = json.loads(args.baseline.read_text(encoding="utf-8"))
    challenger = json.loads(args.challenger.read_text(encoding="utf-8"))
    if baseline["corpus_manifest_sha256"] != challenger["corpus_manifest_sha256"]:
        raise SystemExit("run comparison requires the exact same corpus_manifest_sha256")
    if baseline["harness_revision"] != challenger["harness_revision"]:
        raise SystemExit("run comparison requires the exact same harness_revision")
    if baseline.get("source_accuracy") is None or challenger.get("source_accuracy") is None:
        raise SystemExit("run comparison requires source_accuracy on both artifacts")

    comparison = {
        "schema_version": "health-document-benchmark-comparison-v2",
        "harness_revision": baseline["harness_revision"],
        "corpus_manifest_sha256": baseline["corpus_manifest_sha256"],
        "baseline": {
            "candidate_id": baseline["candidate_id"],
            "configuration_id": baseline["configuration_id"],
            "execution_topology_revision": baseline.get("execution_topology_revision"),
        },
        "challenger": {
            "candidate_id": challenger["candidate_id"],
            "configuration_id": challenger["configuration_id"],
            "execution_topology_revision": challenger.get("execution_topology_revision"),
        },
        "source_accuracy_delta": {
            metric: challenger["source_accuracy"][metric] - baseline["source_accuracy"][metric]
            for metric in SOURCE_METRICS
        },
        "end_to_end_accuracy_delta": {
            metric: challenger["accuracy"][metric] - baseline["accuracy"][metric]
            for metric in END_TO_END_METRICS
        },
        "runtime_ratio": {
            "mean_fixture_ms": (
                challenger["runtime"]["mean_fixture_ms"] / baseline["runtime"]["mean_fixture_ms"]
                if baseline["runtime"]["mean_fixture_ms"]
                else None
            ),
            "cgroup_memory_peak_mb": (
                challenger["runtime"]["cgroup_memory_peak_mb"]
                / baseline["runtime"]["cgroup_memory_peak_mb"]
                if baseline["runtime"].get("cgroup_memory_peak_mb")
                else None
            ),
        },
    }
    rendered = json.dumps(comparison, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
