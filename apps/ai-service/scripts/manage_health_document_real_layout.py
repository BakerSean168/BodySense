"""Manage the private, deidentified real-layout health-document benchmark corpus."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1]
if str(SERVICE_ROOT) not in sys.path:
    sys.path.insert(0, str(SERVICE_ROOT))

from src.document_pipeline.private_corpus import (  # noqa: E402
    DEFAULT_PRIVATE_CORPUS_ROOT,
    PrivateCorpusError,
    import_deidentified_fixture,
    load_truth,
    private_corpus_status,
    review_fixture,
    set_fixture_truth,
    validate_private_selection_corpus,
)


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=DEFAULT_PRIVATE_CORPUS_ROOT)
    sub = parser.add_subparsers(dest="command", required=True)

    add = sub.add_parser("import-fixture")
    add.add_argument("--asset", type=Path, required=True)
    add.add_argument("--truth-json", type=Path, required=True)
    add.add_argument("--fixture-id", required=True)
    add.add_argument("--cohort", required=True)
    add.add_argument("--source-license", default="private-deidentified-evaluation-only")
    add.add_argument("--attest-deidentified", action="store_true")

    truth = sub.add_parser("set-truth")
    truth.add_argument("--fixture-id", required=True)
    truth.add_argument("--truth-json", type=Path, required=True)

    review = sub.add_parser("review")
    review.add_argument("--fixture-id", required=True)
    review.add_argument("--reviewer-id", required=True)
    review.add_argument("--attest-deidentified", action="store_true")

    sub.add_parser("status")
    sub.add_parser("validate")
    return parser


def _privacy_bounded_record(record) -> dict[str, object]:
    return {
        "fixture_id": record.fixture_id,
        "cohort": record.cohort,
        "asset_sha256": record.asset_sha256,
        "truth_sha256": record.truth_sha256(),
        "annotation_state": record.annotation_state,
        "reviewers": [item.reviewer_id for item in record.review_attestations],
        "indicator_count": len(record.indicators),
    }


def main() -> int:
    args = _parser().parse_args()
    try:
        if args.command == "import-fixture":
            record = import_deidentified_fixture(
                source_asset=args.asset,
                truth=load_truth(args.truth_json),
                fixture_id=args.fixture_id,
                cohort=args.cohort,
                human_attests_deidentified=args.attest_deidentified,
                root=args.root,
                source_license=args.source_license,
            )
            result = _privacy_bounded_record(record)
        elif args.command == "set-truth":
            record = set_fixture_truth(
                fixture_id=args.fixture_id,
                truth=load_truth(args.truth_json),
                root=args.root,
            )
            result = _privacy_bounded_record(record)
        elif args.command == "review":
            record = review_fixture(
                fixture_id=args.fixture_id,
                reviewer_id=args.reviewer_id,
                human_attests_deidentified=args.attest_deidentified,
                root=args.root,
            )
            result = _privacy_bounded_record(record)
        elif args.command == "status":
            result = private_corpus_status(args.root)
        elif args.command == "validate":
            result = validate_private_selection_corpus(args.root)
            result["selection_ready"] = True
        else:
            raise AssertionError(f"unhandled command: {args.command}")
    except (PrivateCorpusError, ValueError) as exc:
        print(f"REAL_LAYOUT_CORPUS_ERROR={exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
