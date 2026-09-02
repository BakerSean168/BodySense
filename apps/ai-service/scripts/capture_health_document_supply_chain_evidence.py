"""Capture exact supply-chain evidence for the current health-document candidate."""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from importlib import metadata
from pathlib import Path
from typing import Any

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.configuration.health_document_config import (  # noqa: E402
    get_default_health_document_configuration,
    get_health_document_configuration,
)
from src.document_pipeline.serving_engine import model_root, sha256_file  # noqa: E402

DEFAULT_OUTPUT = (
    SERVICE_ROOT
    / "benchmarks"
    / "health_document"
    / "evidence"
    / "health-document-current-supply-chain.json"
)


def _dist_evidence(name: str) -> dict[str, Any]:
    dist = metadata.distribution(name)
    metadata_entry = next(
        (
            item
            for item in (dist.files or [])
            if item.name == "METADATA" and ".dist-info" in str(item.parent)
        ),
        None,
    )
    if metadata_entry is None:
        raise SystemExit(f"distribution METADATA missing: {name}")
    metadata_path = Path(str(dist.locate_file(metadata_entry)))
    result: dict[str, Any] = {
        "name": name,
        "version": dist.version,
        "metadata_sha256": sha256_file(metadata_path),
        "license": dist.metadata.get("License"),
        "license_expression": dist.metadata.get("License-Expression"),
    }
    return result


def _sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--configuration-id")
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--reviewer-id", default="repository-maintainer")
    parser.add_argument("--approve-production", action="store_true")
    args = parser.parse_args()

    config = (
        get_health_document_configuration(args.configuration_id)
        if args.configuration_id
        else get_default_health_document_configuration()
    )
    if config.ocr_engine != "rapidocr" or config.runtime_engine != "onnxruntime":
        raise SystemExit(
            "supply-chain capture currently supports the RapidOCR/ONNXRuntime candidate"
        )

    rapid = _dist_evidence("rapidocr")
    onnx = _dist_evidence("onnxruntime")
    if rapid["version"] != config.ocr_engine_version:
        raise SystemExit("RapidOCR distribution version does not match configuration")
    if onnx["version"] != config.runtime_version:
        raise SystemExit("ONNXRuntime distribution version does not match configuration")

    import onnxruntime  # pyright: ignore[reportMissingImports]

    onnx_root = Path(onnxruntime.__file__).resolve().parent
    license_path = onnx_root / "LICENSE"
    notice_path = onnx_root / "ThirdPartyNotices.txt"
    if not license_path.is_file() or not notice_path.is_file():
        raise SystemExit("ONNXRuntime license/notice files are missing")
    onnx["license_sha256"] = _sha(license_path)
    onnx["third_party_notices_sha256"] = _sha(notice_path)

    root = model_root()
    models = []
    for artifact in config.model_artifacts:
        path = root / artifact.filename
        if not path.is_file():
            raise SystemExit(f"model artifact missing: {path}")
        actual = sha256_file(path)
        if actual != artifact.sha256:
            raise SystemExit(f"model artifact SHA mismatch: {artifact.filename}")
        models.append(
            {
                "role": artifact.role,
                "filename": artifact.filename,
                "sha256": actual,
                "family": config.model_family,
                "model_type": config.model_type,
            }
        )

    evidence = {
        "schema_version": "health-document-supply-chain-evidence-v1",
        "configuration_id": config.configuration_id,
        "candidate_id": config.mechanism_revision,
        "machine_evidence_complete": True,
        "distributions": [rapid, onnx],
        "model_artifacts": models,
        "official_license_references": [
            {
                "component": "RapidOCR",
                "url": "https://github.com/RapidAI/RapidOCR",
                "license": "Apache-2.0",
            },
            {
                "component": "PaddleOCR / PP-OCRv6 model family",
                "url": "https://github.com/PaddlePaddle/PaddleOCR",
                "license": "Apache-2.0",
            },
            {
                "component": "ONNXRuntime",
                "url": "https://github.com/microsoft/onnxruntime",
                "license": "MIT",
            },
        ],
        "engineering_review": {
            "status": "approved" if args.approve_production else "pending",
            "reviewer_id": args.reviewer_id if args.approve_production else None,
            "scope": "engineering distribution/license compatibility review; not legal advice",
        },
        "production_promotion_ready": bool(args.approve_production),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(
        json.dumps(
            {
                "configuration_id": config.configuration_id,
                "machine_evidence_complete": True,
                "production_promotion_ready": evidence["production_promotion_ready"],
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
