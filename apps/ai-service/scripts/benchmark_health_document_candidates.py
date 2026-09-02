"""Benchmark health-document extraction candidates on one frozen corpus."""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.configuration.health_document_config import (  # noqa: E402
    get_default_health_document_configuration,
    get_health_document_configuration,
)
from src.document_pipeline.baseline import (  # noqa: E402
    load_tesseract_baseline_config,
    verify_tesseract_baseline_runtime,
)
from src.document_pipeline.contracts import (  # noqa: E402
    BenchmarkRunResult,
    FixtureBenchmarkResult,
    PredictedIndicator,
)
from src.document_pipeline.corpus import (  # noqa: E402
    load_corpus_manifest,
    manifest_sha256,
    validate_corpus,
)
from src.document_pipeline.engines.rapidocr_ppocrv6 import (  # noqa: E402
    extract_text_rapidocr,
    load_rapidocr_small_config,
    verify_rapidocr_small_runtime,
)
from src.document_pipeline.engines.rapidocr_ppocrv6_bounded import (  # noqa: E402
    extract_text_rapidocr_bounded,
    load_rapidocr_bounded_config,
    verify_rapidocr_bounded_runtime,
)
from src.document_pipeline.engines.rapidocr_ppocrv6_medium import (  # noqa: E402
    extract_text_rapidocr_medium,
    load_rapidocr_medium_config,
    verify_rapidocr_medium_runtime,
)
from src.document_pipeline.engines.rapidocr_ppocrv6_tiny import (  # noqa: E402
    extract_text_rapidocr_tiny,
    load_rapidocr_tiny_config,
    verify_rapidocr_tiny_runtime,
)
from src.document_pipeline.metrics import (  # noqa: E402
    evaluate_source_text,
    summarize_accuracy,
    summarize_source_accuracy,
)
from src.document_pipeline.runtime_metrics import summarize_runtime  # noqa: E402
from src.document_pipeline.serving_engine import verify_health_document_runtime  # noqa: E402
from src.document_pipeline.subprocess_runner import run_health_document_worker  # noqa: E402
from src.services.indicator_extractor import extract_indicators  # noqa: E402
from src.services.ocr import extract_text as extract_tesseract_text  # noqa: E402

DEFAULT_CORPUS_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "synthetic-v1"
DEFAULT_OUTPUT_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "runs"
SUPPORTED_CANDIDATES = (
    "health-document-current",
    "tesseract",
    "rapidocr-ppocrv6-small",
    "rapidocr-ppocrv6-small-bounded",
    "rapidocr-ppocrv6-tiny",
    "rapidocr-ppocrv6-medium",
)


@dataclass(frozen=True)
class CandidateIdentity:
    candidate_id: str
    configuration_id: str
    fingerprint: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidate", choices=SUPPORTED_CANDIDATES, default="tesseract")
    parser.add_argument("--corpus-root", type=Path, default=DEFAULT_CORPUS_ROOT)
    parser.add_argument("--cohort", action="append")
    parser.add_argument("--fixture", action="append")
    parser.add_argument("--output", type=Path)
    parser.add_argument(
        "--configuration-id",
        help=(
            "Exact serving configuration for health-document-current; defaults to repository target"
        ),
    )
    parser.add_argument("--production-shaped", action="store_true")
    parser.add_argument(
        "--execution-topology",
        choices=("in-process-v1", "per-document-subprocess-v1"),
        default="in-process-v1",
    )
    parser.add_argument("--fixture-worker", action="store_true", help=argparse.SUPPRESS)
    parser.add_argument("--worker-output", type=Path, help=argparse.SUPPRESS)
    parser.add_argument(
        "--skip-runtime-identity-check",
        action="store_true",
        help="Test-only escape hatch; never use for qualification artifacts",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    corpus_root = args.corpus_root.resolve()
    manifest_path = corpus_root / "corpus_manifest.jsonl"
    records = load_corpus_manifest(manifest_path)
    validate_corpus(records, asset_root=corpus_root, require_minimum_shape=True)

    if args.cohort:
        selected = set(args.cohort)
        records = [record for record in records if record.cohort in selected]
    if args.fixture:
        selected = set(args.fixture)
        records = [record for record in records if record.fixture_id in selected]
    if not records:
        raise SystemExit("no benchmark fixtures selected")

    serving_candidate = args.candidate == "health-document-current"
    if serving_candidate:
        serving_config = (
            get_health_document_configuration(args.configuration_id)
            if args.configuration_id
            else get_default_health_document_configuration()
        )
        if not args.skip_runtime_identity_check:
            verify_health_document_runtime(serving_config)
        config = CandidateIdentity(
            candidate_id=serving_config.mechanism_revision,
            configuration_id=serving_config.configuration_id,
            fingerprint=serving_config.fingerprint,
        )
        extractor = None
        if args.execution_topology != "per-document-subprocess-v1" and not args.fixture_worker:
            raise SystemExit(
                "health-document-current qualification requires "
                "--execution-topology per-document-subprocess-v1"
            )
    elif args.candidate == "tesseract":
        config = load_tesseract_baseline_config()
        if not args.skip_runtime_identity_check:
            verify_tesseract_baseline_runtime(config)
        extractor = _extract_tesseract
    elif args.candidate == "rapidocr-ppocrv6-small":
        config = load_rapidocr_small_config()
        if not args.skip_runtime_identity_check:
            verify_rapidocr_small_runtime(config)
        extractor = extract_text_rapidocr
    elif args.candidate == "rapidocr-ppocrv6-small-bounded":
        config = load_rapidocr_bounded_config()
        if not args.skip_runtime_identity_check:
            verify_rapidocr_bounded_runtime(config)
        extractor = extract_text_rapidocr_bounded
    elif args.candidate == "rapidocr-ppocrv6-tiny":
        config = load_rapidocr_tiny_config()
        if not args.skip_runtime_identity_check:
            verify_rapidocr_tiny_runtime(config)
        extractor = extract_text_rapidocr_tiny
    elif args.candidate == "rapidocr-ppocrv6-medium":
        config = load_rapidocr_medium_config()
        if not args.skip_runtime_identity_check:
            verify_rapidocr_medium_runtime(config)
        extractor = extract_text_rapidocr_medium
    else:
        raise SystemExit(f"candidate not implemented yet: {args.candidate}")

    if args.fixture_worker:
        if len(records) != 1 or args.worker_output is None:
            raise SystemExit("fixture worker requires exactly one --fixture and --worker-output")
        if serving_candidate:
            result = _run_serving_fixture(
                records[0],
                asset_root=corpus_root,
                configuration_id=config.configuration_id,
            )
        else:
            assert extractor is not None
            result = _run_fixture(records[0], asset_root=corpus_root, extractor=extractor)
        args.worker_output.write_text(
            json.dumps(result.model_dump(mode="json"), ensure_ascii=False, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        return 0

    if args.execution_topology == "per-document-subprocess-v1":
        if serving_candidate:
            fixture_results = [
                _run_serving_fixture(
                    record,
                    asset_root=corpus_root,
                    configuration_id=config.configuration_id,
                )
                for record in records
            ]
        else:
            fixture_results = [
                _run_isolated_fixture(
                    candidate=args.candidate,
                    fixture_id=record.fixture_id,
                    corpus_root=corpus_root,
                    configuration_id=None,
                )
                for record in records
            ]
    else:
        if serving_candidate:
            raise SystemExit("health-document-current cannot run in-process for qualification")
        assert extractor is not None
        fixture_results = [
            _run_fixture(record, asset_root=corpus_root, extractor=extractor) for record in records
        ]
    predictions = {
        result.fixture_id: result.predicted_indicators
        for result in fixture_results
        if result.error is None
    }
    accuracy = summarize_accuracy(records, predictions)
    run = BenchmarkRunResult(
        schema_version="health-document-benchmark-run-v1",
        harness_revision="health-document-benchmark-v4",
        candidate_id=config.candidate_id,
        configuration_id=config.configuration_id,
        candidate_fingerprint=config.fingerprint,
        corpus_manifest_sha256=manifest_sha256(manifest_path),
        production_shaped=args.production_shaped,
        execution_topology_revision=(
            serving_config.execution_topology_revision
            if serving_config is not None
            else args.execution_topology
        ),
        fixture_results=fixture_results,
        source_accuracy=summarize_source_accuracy(fixture_results),
        accuracy=accuracy,
        runtime=summarize_runtime(fixture_results),
    )

    output = args.output
    if output is None:
        DEFAULT_OUTPUT_ROOT.mkdir(parents=True, exist_ok=True)
        output = DEFAULT_OUTPUT_ROOT / f"{config.candidate_id}.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(run.model_dump(mode="json"), ensure_ascii=False, indent=2, sort_keys=True)
        + "\n",
        encoding="utf-8",
    )
    print(json.dumps(accuracy.model_dump(), ensure_ascii=False, sort_keys=True))
    print(json.dumps(run.runtime.model_dump(), ensure_ascii=False, sort_keys=True))
    print(f"configuration_id={config.configuration_id}")
    print(f"output={output}")
    return 0


def _extract_tesseract(file_bytes: bytes, mime_type: str) -> str:
    text, _ = extract_tesseract_text(file_bytes, mime_type)
    return text


def _run_fixture(
    document,
    *,
    asset_root: Path,
    extractor: Callable[[bytes, str], str],
) -> FixtureBenchmarkResult:
    asset_path = (asset_root / document.asset_path).resolve()
    started = time.perf_counter()
    raw_text = extractor(asset_path.read_bytes(), document.mime_type)
    indicators = extract_indicators(raw_text)
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    return FixtureBenchmarkResult(
        fixture_id=document.fixture_id,
        cohort=document.cohort,
        elapsed_ms=elapsed_ms,
        raw_text_sha256=hashlib.sha256(raw_text.encode()).hexdigest(),
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
            for indicator in indicators
        ],
        source_counts=evaluate_source_text(document, raw_text),
    )


def _run_serving_fixture(
    document,
    *,
    asset_root: Path,
    configuration_id: str,
) -> FixtureBenchmarkResult:
    asset_path = (asset_root / document.asset_path).resolve()
    started = time.perf_counter()
    response = asyncio.run(
        run_health_document_worker(
            asset_path.read_bytes(),
            document.mime_type,
            configuration_id,
        )
    )
    elapsed_ms = (time.perf_counter() - started) * 1000.0
    raw_text = response.result.raw_text
    return FixtureBenchmarkResult(
        fixture_id=document.fixture_id,
        cohort=document.cohort,
        elapsed_ms=elapsed_ms,
        raw_text_sha256=hashlib.sha256(raw_text.encode()).hexdigest(),
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
        source_counts=evaluate_source_text(document, raw_text),
    )


def _run_isolated_fixture(
    *,
    candidate: str,
    fixture_id: str,
    corpus_root: Path,
    configuration_id: str | None = None,
) -> FixtureBenchmarkResult:
    with tempfile.TemporaryDirectory(prefix="bodysense-hd-fixture-") as tmp_dir:
        output = Path(tmp_dir) / "fixture.json"
        command = [
            sys.executable,
            str(Path(__file__).resolve()),
            "--candidate",
            candidate,
            "--corpus-root",
            str(corpus_root),
            "--fixture",
            fixture_id,
            "--fixture-worker",
            "--worker-output",
            str(output),
            "--skip-runtime-identity-check",
        ]
        if configuration_id is not None:
            command.extend(["--configuration-id", configuration_id])
        started = time.perf_counter()
        completed = subprocess.run(command, check=False)
        elapsed_ms = (time.perf_counter() - started) * 1000.0
        if completed.returncode != 0 or not output.is_file():
            return FixtureBenchmarkResult(
                fixture_id=fixture_id,
                cohort=_fixture_cohort(corpus_root, fixture_id),
                elapsed_ms=elapsed_ms,
                raw_text_sha256="0" * 64,
                predicted_indicators=[],
                source_counts=None,
                error=f"isolated_fixture_exit_{completed.returncode}",
            )
        return FixtureBenchmarkResult.model_validate_json(output.read_text(encoding="utf-8"))


def _fixture_cohort(corpus_root: Path, fixture_id: str):
    records = load_corpus_manifest(corpus_root / "corpus_manifest.jsonl")
    for record in records:
        if record.fixture_id == fixture_id:
            return record.cohort
    raise ValueError(f"unknown fixture_id: {fixture_id}")


if __name__ == "__main__":
    raise SystemExit(main())
