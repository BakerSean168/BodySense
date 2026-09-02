#!/usr/bin/env python3
"""Provision exact health-document OCR model artifacts outside request paths."""

from __future__ import annotations

import argparse
import shutil
import sys
import tempfile
from importlib import metadata
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.configuration.health_document_config import (  # noqa: E402
    get_default_health_document_configuration,
)
from src.document_pipeline.serving_engine import sha256_file  # noqa: E402

DEFAULT_OUTPUT = SERVICE_ROOT / "models" / "health-document" / "ppocrv6-small-v1"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    args = parser.parse_args()
    config = get_default_health_document_configuration()
    if metadata.version("rapidocr") != config.ocr_engine_version:
        raise SystemExit("RapidOCR package version does not match health-document manifest")
    if metadata.version("onnxruntime") != config.runtime_version:
        raise SystemExit("ONNXRuntime package version does not match health-document manifest")

    import rapidocr  # pyright: ignore[reportMissingImports]

    package_models = Path(rapidocr.__file__).resolve().parent / "models"
    target_root = args.output.expanduser().resolve()
    target_root.mkdir(parents=True, exist_ok=True)
    for artifact in config.model_artifacts:
        source = package_models / artifact.filename
        if not source.is_file():
            raise SystemExit(f"RapidOCR package model missing: {source}")
        source_hash = sha256_file(source)
        if source_hash != artifact.sha256:
            raise SystemExit(
                f"RapidOCR package model SHA256 mismatch for {artifact.filename}: "
                f"got {source_hash} want {artifact.sha256}"
            )
        target = target_root / artifact.filename
        if target.is_file() and sha256_file(target) == artifact.sha256:
            continue
        with tempfile.NamedTemporaryFile(
            prefix=artifact.filename + ".",
            suffix=".partial",
            dir=target_root,
            delete=False,
        ) as handle:
            tmp = Path(handle.name)
        try:
            shutil.copyfile(source, tmp)
            copied_hash = sha256_file(tmp)
            if copied_hash != artifact.sha256:
                raise SystemExit(
                    f"copied model SHA256 mismatch for {artifact.filename}: {copied_hash}"
                )
            tmp.replace(target)
        finally:
            tmp.unlink(missing_ok=True)
    print(
        "HEALTH_DOCUMENT_MODELS_READY "
        f"configuration_id={config.configuration_id} path={target_root}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
