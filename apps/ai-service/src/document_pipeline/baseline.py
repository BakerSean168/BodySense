"""Current production Tesseract benchmark baseline identity and verification."""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import subprocess
from importlib import metadata
from pathlib import Path

from .contracts import BenchmarkCandidateConfig

SERVICE_ROOT = Path(__file__).resolve().parents[2]
BENCHMARK_ROOT = SERVICE_ROOT / "benchmarks" / "health_document"
TESSERACT_BASELINE_PATH = BENCHMARK_ROOT / "candidates" / "tesseract-production-v0.10.2.json"


class CandidateUnavailableError(RuntimeError):
    """The benchmark candidate cannot execute in the current environment."""


class CandidateIdentityMismatchError(RuntimeError):
    """The current runtime does not match the declared immutable candidate identity."""


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_tesseract_baseline_config() -> BenchmarkCandidateConfig:
    payload = json.loads(TESSERACT_BASELINE_PATH.read_text(encoding="utf-8"))
    return BenchmarkCandidateConfig.model_validate(payload)


def _tesseract_version() -> str:
    executable = shutil.which("tesseract")
    if executable is None:
        raise CandidateUnavailableError("tesseract executable is not installed")
    proc = subprocess.run(
        [executable, "--version"],
        check=True,
        capture_output=True,
        text=True,
    )
    first_line = proc.stdout.splitlines()[0].strip() if proc.stdout else ""
    match = re.match(r"tesseract\s+(.+)$", first_line)
    if match is None:
        raise CandidateIdentityMismatchError(
            f"unable to parse tesseract version from {first_line!r}"
        )
    return match.group(1)


def _tessdata_dir() -> Path:
    executable = shutil.which("tesseract")
    if executable is None:
        raise CandidateUnavailableError("tesseract executable is not installed")
    proc = subprocess.run(
        [executable, "--list-langs"],
        check=True,
        capture_output=True,
        text=True,
    )
    combined = "\n".join(part for part in (proc.stdout, proc.stderr) if part)
    match = re.search(r'List of available languages in "([^"]+)"', combined)
    if match is not None:
        return Path(match.group(1))
    prefix = os.getenv("TESSDATA_PREFIX", "").strip()
    if prefix:
        return Path(prefix)
    raise CandidateIdentityMismatchError("unable to resolve Tesseract tessdata directory")


def _assert_equal(label: str, actual: str, expected: str) -> None:
    if actual != expected:
        raise CandidateIdentityMismatchError(f"{label} mismatch: got {actual!r} want {expected!r}")


def verify_tesseract_baseline_source_contract(
    config: BenchmarkCandidateConfig | None = None,
) -> dict[str, str]:
    """Verify repository-owned extraction/parser source hashes independently of OCR runtime."""

    config = config or load_tesseract_baseline_config()
    source_paths = {
        "ocr_service_sha256": SERVICE_ROOT / "src" / "services" / "ocr.py",
        "indicator_extractor_sha256": SERVICE_ROOT / "src" / "services" / "indicator_extractor.py",
        "admissibility_policy_sha256": SERVICE_ROOT
        / "src"
        / "services"
        / "report_indicator_admissibility_v1.py",
    }
    expected_sources = config.source_contract.model_dump()
    verified: dict[str, str] = {}
    for key, path in source_paths.items():
        actual = sha256_file(path)
        _assert_equal(key, actual, str(expected_sources[key]))
        verified[key] = actual
    return verified


def verify_tesseract_baseline_runtime(
    config: BenchmarkCandidateConfig | None = None,
) -> dict[str, str]:
    """Fail closed unless this runtime is the exact frozen production baseline."""

    config = config or load_tesseract_baseline_config()
    verify_tesseract_baseline_source_contract(config)
    _assert_equal("tesseract version", _tesseract_version(), config.engine_version)
    _assert_equal("pytesseract version", metadata.version("pytesseract"), config.wrapper_version)
    _assert_equal("PyMuPDF version", metadata.version("PyMuPDF"), config.pdf_raster.engine_version)
    _assert_equal("Pillow version", metadata.version("Pillow"), config.pillow_version)

    tessdata_dir = _tessdata_dir()
    for artifact in config.language_artifacts:
        path = tessdata_dir / f"{artifact.language}.traineddata"
        if not path.is_file():
            raise CandidateUnavailableError(f"missing Tesseract language artifact: {path}")
        _assert_equal(
            f"{artifact.language} traineddata sha256",
            sha256_file(path),
            artifact.sha256,
        )

    return {
        "status": "verified",
        "candidate_id": config.candidate_id,
        "configuration_id": config.configuration_id,
        "fingerprint": config.fingerprint,
        "engine": config.engine,
        "engine_version": config.engine_version,
        "languages": "+".join(config.languages),
        "tessdata_dir": str(tessdata_dir),
    }
