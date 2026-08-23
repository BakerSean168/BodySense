"""Materialize an immutable reviewed-knowledge projection from explicit review manifests."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from src.rag.claim_review import (  # noqa: E402
    ReviewedKnowledgeUnit,
    build_reviewed_snapshot,
    load_claim_review,
)
from src.rag.external_evidence import load_external_evidence_review  # noqa: E402
from src.rag.thought_forest_snapshot import (  # noqa: E402
    build_generated_packs,
    load_thought_forest_snapshot,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("snapshot")
    parser.add_argument("--evidence-review-manifest", required=True)
    parser.add_argument("--claim-review-manifest", required=True)
    parser.add_argument("--output", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    snapshot = load_thought_forest_snapshot(args.snapshot)
    evidence_review = load_external_evidence_review(args.evidence_review_manifest)
    claim_review = load_claim_review(args.claim_review_manifest)
    packs = build_generated_packs(
        snapshot,
        review_manifest=evidence_review,
        claim_review_manifest=claim_review,
    )

    reviewed_units: list[ReviewedKnowledgeUnit] = []
    for pack in packs:
        for unit in pack.units:
            claim_review_payload = dict(unit.metadata.get("claim_review") or {})
            if not claim_review_payload:
                continue
            claim_candidate = dict(unit.metadata.get("claim_candidate") or {})
            claim_admissibility = dict(unit.metadata.get("claim_admissibility") or {})
            reviewed_units.append(
                ReviewedKnowledgeUnit(
                    unit_key=unit.unit_key,
                    claim_id=str(claim_candidate.get("claim_id") or ""),
                    claim_content_hash=str(unit.content_hash or ""),
                    review_status=unit.review_status,
                    lifecycle_status=unit.lifecycle_status,
                    quality_score=unit.quality_score,
                    publication_eligible=(
                        claim_admissibility.get("publication_eligible") is True
                    ),
                    source_locator=dict(unit.metadata.get("source_locator") or {}),
                    claim_review=claim_review_payload,
                )
            )

    if not reviewed_units:
        raise RuntimeError("claim review produced no reviewed knowledge units")

    reviewed = build_reviewed_snapshot(
        source_snapshot_id=snapshot.snapshot_id,
        source_git_commit=snapshot.repository.git_commit,
        external_evidence_review_id=evidence_review.review_id,
        claim_review_id=claim_review.review_id,
        units=reviewed_units,
    )
    output = Path(args.output).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        json.dumps(reviewed.model_dump(), ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"Reviewed snapshot: {reviewed.reviewed_snapshot_id}")
    print(f"Reviewed units: {len(reviewed.units)}")
    print(f"Output: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
