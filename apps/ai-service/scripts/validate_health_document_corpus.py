"""Validate the BodySense health-document benchmark corpus contract."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.document_pipeline.corpus import load_corpus_manifest, validate_corpus  # noqa: E402

DEFAULT_ROOT = SERVICE_ROOT / "data" / "benchmarks" / "health_document" / "synthetic-v1"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=DEFAULT_ROOT)
    parser.add_argument(
        "--selection-ready",
        action="store_true",
        help=(
            "Also require deidentified + double-reviewed real-layout evidence "
            "before Champion selection"
        ),
    )
    args = parser.parse_args()
    root = args.root.resolve()
    manifest = root / "corpus_manifest.jsonl"
    records = load_corpus_manifest(manifest)
    summary = validate_corpus(
        records,
        asset_root=root,
        require_minimum_shape=True,
        require_selection_ready=args.selection_ready,
    )
    print(json.dumps(summary.__dict__, ensure_ascii=False, indent=2, default=list))
    if not args.selection_ready and summary.deidentified_documents == 0:
        print("SELECTION_READY=false reason=deidentified_double_reviewed_subset_missing")
    else:
        print("SELECTION_READY=true")
    print("CORPUS_VALIDATION=PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
