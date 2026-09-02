"""Benchmark the real Health Document HTTP runtime from outside its cgroup."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import subprocess
import sys
import time
from pathlib import Path

import httpx

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.configuration.health_document_config import get_health_document_configuration  # noqa: E402
from src.document_pipeline.contracts import (  # noqa: E402
    BenchmarkRunResult,
    FixtureBenchmarkResult,
    PredictedIndicator,
    RuntimeSummary,
)
from src.document_pipeline.corpus import (  # noqa: E402
    load_corpus_manifest,
    manifest_sha256,
    validate_corpus,
    validate_real_layout_selection_subset,
)
from src.document_pipeline.metrics import (  # noqa: E402
    evaluate_source_text,
    summarize_accuracy,
    summarize_source_accuracy,
)
from src.models.ocr import OCRResponse  # noqa: E402

DEFAULT_CORPUS_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "synthetic-v1"
DEFAULT_OUTPUT_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "runs"


def _p95(values: list[float]) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * 0.95) - 1)]


def _docker_cat(container: str, path: str) -> str:
    completed = subprocess.run(
        ["docker", "exec", container, "cat", path],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    if completed.returncode != 0:
        raise RuntimeError(f"failed to read {path} from runtime container")
    return completed.stdout.strip()


def _runtime_summary(container: str, elapsed: list[float]) -> RuntimeSummary:
    memory_max = _docker_cat(container, "/sys/fs/cgroup/memory.max")
    memory_peak = _docker_cat(container, "/sys/fs/cgroup/memory.peak")
    swap_peak = _docker_cat(container, "/sys/fs/cgroup/memory.swap.peak")
    events = {}
    for line in _docker_cat(container, "/sys/fs/cgroup/memory.events").splitlines():
        fields = line.split()
        if len(fields) == 2:
            events[fields[0]] = int(fields[1])
    cpu_fields = _docker_cat(container, "/sys/fs/cgroup/cpu.max").split()
    cpu_limit = None
    if len(cpu_fields) == 2 and cpu_fields[0] != "max":
        cpu_limit = int(cpu_fields[0]) / int(cpu_fields[1])
    return RuntimeSummary(
        total_elapsed_ms=sum(elapsed),
        mean_fixture_ms=sum(elapsed) / len(elapsed) if elapsed else 0.0,
        p95_fixture_ms=_p95(elapsed),
        peak_self_rss_mb=0.0,
        peak_child_rss_mb=0.0,
        cgroup_memory_limit_mb=(None if memory_max == "max" else int(memory_max) / (1024 * 1024)),
        cgroup_memory_peak_mb=int(memory_peak) / (1024 * 1024),
        cgroup_memory_events_max=events.get("max"),
        cgroup_memory_events_oom=events.get("oom"),
        cgroup_memory_events_oom_kill=events.get("oom_kill"),
        cgroup_swap_peak_mb=int(swap_peak) / (1024 * 1024),
        cgroup_cpu_limit=cpu_limit,
    )


def _fixture_result(record, response: OCRResponse, elapsed_ms: float) -> FixtureBenchmarkResult:
    return FixtureBenchmarkResult(
        fixture_id=record.fixture_id,
        cohort=record.cohort,
        elapsed_ms=elapsed_ms,
        raw_text_sha256=hashlib.sha256(response.result.raw_text.encode()).hexdigest(),
        predicted_indicators=[
            PredictedIndicator(
                name=indicator.name,
                value=indicator.value,
                unit=indicator.unit,
                reference_range=indicator.reference_range,
                confidence=indicator.confidence,
                admissibility_status=indicator.evidence_admissibility.status,
                verification_status=(
                    indicator.evidence_verification.status
                    if indicator.evidence_verification is not None
                    else None
                ),
            )
            for indicator in response.result.indicators
        ],
        source_counts=evaluate_source_text(record, response.result.raw_text),
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--configuration-id", required=True)
    parser.add_argument("--runtime-container", required=True)
    parser.add_argument("--corpus-root", type=Path, default=DEFAULT_CORPUS_ROOT)
    parser.add_argument("--cohort", action="append")
    parser.add_argument("--fixture", action="append")
    parser.add_argument("--output", type=Path)
    parser.add_argument(
        "--real-layout-selection",
        action="store_true",
        help="Require the private deidentified double-reviewed selection-subset contract.",
    )
    args = parser.parse_args()

    config = get_health_document_configuration(args.configuration_id)
    corpus_root = args.corpus_root.resolve()
    manifest_path = corpus_root / "corpus_manifest.jsonl"
    records = load_corpus_manifest(manifest_path)
    if args.real_layout_selection:
        validate_real_layout_selection_subset(records, asset_root=corpus_root)
    else:
        validate_corpus(records, asset_root=corpus_root, require_minimum_shape=True)
    if args.cohort:
        selected = set(args.cohort)
        records = [record for record in records if record.cohort in selected]
    if args.fixture:
        selected = set(args.fixture)
        records = [record for record in records if record.fixture_id in selected]
    if not records:
        raise SystemExit("no HTTP benchmark fixtures selected")

    fixture_results: list[FixtureBenchmarkResult] = []
    elapsed_values: list[float] = []
    endpoint = args.base_url.rstrip("/") + "/api/ocr/extract"
    with httpx.Client(timeout=180.0) as client:
        for record in records:
            asset_path = (corpus_root / record.asset_path).resolve()
            started = time.perf_counter()
            try:
                response = client.post(
                    endpoint,
                    data={"configuration_id": config.configuration_id},
                    files={"file": ("upload", asset_path.read_bytes(), record.mime_type)},
                )
                elapsed_ms = (time.perf_counter() - started) * 1000.0
                elapsed_values.append(elapsed_ms)
                if response.status_code != 200:
                    fixture_results.append(
                        FixtureBenchmarkResult(
                            fixture_id=record.fixture_id,
                            cohort=record.cohort,
                            elapsed_ms=elapsed_ms,
                            raw_text_sha256="0" * 64,
                            predicted_indicators=[],
                            error=f"http_status_{response.status_code}",
                        )
                    )
                    continue
                parsed = OCRResponse.model_validate(response.json())
                if parsed.result.mechanism_provenance is None or (
                    parsed.result.mechanism_provenance.configuration_id != config.configuration_id
                ):
                    raise RuntimeError("HTTP runtime returned the wrong configuration identity")
                fixture_results.append(_fixture_result(record, parsed, elapsed_ms))
            except Exception as exc:
                elapsed_ms = (time.perf_counter() - started) * 1000.0
                elapsed_values.append(elapsed_ms)
                fixture_results.append(
                    FixtureBenchmarkResult(
                        fixture_id=record.fixture_id,
                        cohort=record.cohort,
                        elapsed_ms=elapsed_ms,
                        raw_text_sha256="0" * 64,
                        predicted_indicators=[],
                        error=type(exc).__name__,
                    )
                )

    predictions = {
        result.fixture_id: result.predicted_indicators
        for result in fixture_results
        if result.error is None
    }
    run = BenchmarkRunResult(
        schema_version="health-document-benchmark-run-v1",
        harness_revision="health-document-benchmark-v4",
        candidate_id=config.mechanism_revision,
        configuration_id=config.configuration_id,
        candidate_fingerprint=config.fingerprint,
        corpus_manifest_sha256=manifest_sha256(manifest_path),
        production_shaped=True,
        execution_topology_revision=config.execution_topology_revision,
        fixture_results=fixture_results,
        source_accuracy=summarize_source_accuracy(fixture_results),
        accuracy=summarize_accuracy(records, predictions),
        runtime=_runtime_summary(args.runtime_container, elapsed_values),
    )
    output = args.output or (DEFAULT_OUTPUT_ROOT / f"{config.configuration_id}-http.json")
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(run.model_dump(mode="json"), ensure_ascii=False, indent=2, sort_keys=True)
        + "\n",
        encoding="utf-8",
    )
    print(json.dumps(run.accuracy.model_dump(), ensure_ascii=False, sort_keys=True))
    print(json.dumps(run.runtime.model_dump(), ensure_ascii=False, sort_keys=True))
    print(f"configuration_id={config.configuration_id}")
    print(f"output={output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
