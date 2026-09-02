"""Short-lived independent verifier for OCR-derived health-document pages."""

from __future__ import annotations

import argparse
import hashlib
import sys
from pathlib import Path

from ..configuration.health_document_config import get_health_document_configuration
from .row_verifier import verify_document_pages

MAX_WORKER_INPUT_BYTES = 10 * 1024 * 1024


def verifier_worker_source_sha256() -> str:
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def _parse_pages(value: str) -> list[int]:
    pages = sorted({int(item) for item in value.split(",") if item.strip()})
    if any(page < 1 for page in pages):
        raise ValueError("verification pages must be positive")
    return pages


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--configuration-id", required=True)
    parser.add_argument("--mime-type", required=True)
    parser.add_argument("--pages", required=True)
    args = parser.parse_args()

    payload = sys.stdin.buffer.read(MAX_WORKER_INPUT_BYTES + 1)
    if len(payload) > MAX_WORKER_INPUT_BYTES:
        raise SystemExit("health-document verifier input exceeds 10 MiB")
    config = get_health_document_configuration(args.configuration_id)
    response = verify_document_pages(
        payload,
        args.mime_type,
        config,
        _parse_pages(args.pages),
    )
    sys.stdout.write(response.model_dump_json())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
