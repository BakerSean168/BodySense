"""Produce a privacy-bounded aggregate summary from a health-document benchmark run."""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.document_pipeline.contracts import BenchmarkRunResult  # noqa: E402
from src.document_pipeline.corpus import (  # noqa: E402
    load_corpus_manifest,
    manifest_sha256,
    validate_real_layout_selection_subset,
)
from src.document_pipeline.metrics import (  # noqa: E402
    canonical_indicator_id,
    summarize_accuracy,
    summarize_evidence_authority,
    summarize_source_accuracy,
)


def _norm(value: str | None) -> str:
    if value is None:
        return ""
    return "".join(value.lower().replace("～", "-").replace("~", "-").split())


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", type=Path, required=True)
    parser.add_argument("--corpus-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    run = BenchmarkRunResult.model_validate_json(args.run.read_text(encoding="utf-8"))
    manifest = args.corpus_root / "corpus_manifest.jsonl"
    records = load_corpus_manifest(manifest)
    exact_manifest_sha = manifest_sha256(manifest)
    if run.corpus_manifest_sha256 != exact_manifest_sha:
        raise SystemExit("run artifact does not match the supplied exact corpus manifest")

    run_by_fixture = {result.fixture_id: result for result in run.fixture_results}
    prediction_map = {
        fixture_id: result.predicted_indicators for fixture_id, result in run_by_fixture.items()
    }
    selected_records = [record for record in records if record.fixture_id in run_by_fixture]
    records_by_cohort: dict[str, list] = defaultdict(list)
    for record in selected_records:
        records_by_cohort[record.cohort].append(record)

    cohort_accuracy = {
        cohort: summarize_accuracy(items, prediction_map).model_dump(mode="json")
        for cohort, items in sorted(records_by_cohort.items())
    }
    results_by_cohort = {
        cohort: [run_by_fixture[item.fixture_id] for item in items]
        for cohort, items in sorted(records_by_cohort.items())
    }
    cohort_source_accuracy = {
        cohort: summarize_source_accuracy(results).model_dump(mode="json")
        for cohort, results in results_by_cohort.items()
    }

    failures: Counter[str] = Counter()
    for record in records:
        result = run_by_fixture.get(record.fixture_id)
        if result is None:
            continue
        predicted_by_id = {}
        for prediction in result.predicted_indicators:
            indicator_id = canonical_indicator_id(prediction.name)
            if indicator_id is None:
                failures["unmapped_prediction"] += 1
                continue
            if indicator_id in predicted_by_id:
                failures["duplicate_canonical_prediction"] += 1
                continue
            predicted_by_id[indicator_id] = prediction

        for truth in record.indicators:
            prediction = predicted_by_id.get(truth.indicator_id)
            if prediction is None:
                failures["missing_indicator"] += 1
                continue
            if _norm(prediction.value) != _norm(truth.value):
                failures["numeric_mismatch"] += 1
            if _norm(prediction.unit) != _norm(truth.unit):
                failures["unit_mismatch"] += 1
            if _norm(prediction.reference_range) != _norm(truth.reference_range):
                failures["reference_range_mismatch"] += 1

    failed_fixtures = [result for result in run.fixture_results if result.error is not None]
    real_layout_only = bool(selected_records) and all(
        record.source_classification == "deidentified" for record in selected_records
    )
    real_layout_selection_ready = False
    if real_layout_only:
        try:
            validate_real_layout_selection_subset(selected_records, asset_root=args.corpus_root)
            real_layout_selection_ready = True
        except ValueError:
            real_layout_selection_ready = False
    selection_scope = (
        "deidentified-double-reviewed-real-layout"
        if real_layout_only
        else "synthetic-mechanics-only"
    )
    selection_blocker = (
        None
        if real_layout_selection_ready
        else "real_layout_selection_subset_not_ready"
        if real_layout_only
        else "deidentified_double_reviewed_real-layout_subset_missing"
    )
    summary = {
        "schema_version": "health-document-benchmark-summary-v1",
        "privacy": {
            "contains_raw_text": False,
            "contains_document_images": False,
            "contains_user_identifiers": False,
        },
        "candidate_id": run.candidate_id,
        "configuration_id": run.configuration_id,
        "candidate_fingerprint": run.candidate_fingerprint,
        "harness_revision": run.harness_revision,
        "corpus_manifest_sha256": run.corpus_manifest_sha256,
        "production_shaped": run.production_shaped,
        "execution_topology_revision": run.execution_topology_revision,
        "source_accuracy": (
            run.source_accuracy.model_dump(mode="json") if run.source_accuracy is not None else None
        ),
        "accuracy": run.accuracy.model_dump(mode="json"),
        "evidence_authority": summarize_evidence_authority(
            selected_records, prediction_map
        ).model_dump(mode="json"),
        "runtime": run.runtime.model_dump(mode="json"),
        "fixture_execution": {
            "total": len(run.fixture_results),
            "succeeded": len(run.fixture_results) - len(failed_fixtures),
            "failed": len(failed_fixtures),
        },
        "cohort_source_accuracy": cohort_source_accuracy,
        "cohort_accuracy": cohort_accuracy,
        "failure_taxonomy": dict(sorted(failures.items())),
        "selection_evidence_scope": selection_scope,
        "champion_selection_ready": real_layout_selection_ready,
        "champion_selection_blocker": selection_blocker,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps(summary["accuracy"], sort_keys=True))
    print(f"summary={args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
