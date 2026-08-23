"""Inspect external-evidence resolution and claim admissibility for a Thought Forest snapshot."""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from src.rag.external_evidence import load_external_evidence_review  # noqa: E402
from src.rag.thought_forest_snapshot import (  # noqa: E402
    build_generated_packs,
    load_thought_forest_snapshot,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("snapshot", help="Path to bodysense.health.snapshot.v1/v2/v3 JSON")
    parser.add_argument("--evidence-review-manifest")
    parser.add_argument("--json", action="store_true", help="Emit machine-readable JSON")
    return parser.parse_args()


def summarize(
    snapshot_path: str | Path,
    evidence_review_manifest: str | Path | None = None,
) -> dict[str, object]:
    snapshot = load_thought_forest_snapshot(snapshot_path)
    review_manifest = (
        load_external_evidence_review(evidence_review_manifest)
        if evidence_review_manifest
        else None
    )
    packs = build_generated_packs(snapshot, review_manifest=review_manifest)

    direct_candidates: list[dict] = []
    bibliography_candidates: list[dict] = []
    admissibility: list[dict] = []
    for pack in packs:
        bibliography_candidates.extend(
            list(pack.source.metadata.get("bibliography_reference_candidates") or [])
        )
        for unit in pack.units:
            direct_candidates.extend(list(unit.metadata.get("external_evidence_candidates") or []))
            if unit.metadata.get("claim_admissibility"):
                admissibility.append(dict(unit.metadata["claim_admissibility"]))

    provider_counts = Counter(str(item.get("provider") or "unknown") for item in direct_candidates)
    provider_counts.update(
        str(item.get("provider") or "unknown") for item in bibliography_candidates
    )
    source_type_counts = Counter(
        str(item.get("source_type") or "unknown")
        for item in [*direct_candidates, *bibliography_candidates]
    )
    blocking_reason_counts = Counter(
        str(reason)
        for item in admissibility
        for reason in item.get("blocking_reasons", [])
    )

    return {
        "schema_version": snapshot.schema_version,
        "snapshot_id": snapshot.snapshot_id,
        "git_commit": snapshot.repository.git_commit,
        "notes": len(packs),
        "units": sum(len(pack.units) for pack in packs),
        "direct_reference_candidates": len(direct_candidates),
        "bibliography_reference_candidates": len(bibliography_candidates),
        "provider_counts": dict(sorted(provider_counts.items())),
        "source_type_counts": dict(sorted(source_type_counts.items())),
        "claim_admissibility": {
            "total": len(admissibility),
            "evidence_ready_for_claim_review": sum(
                1
                for item in admissibility
                if item.get("evidence_ready_for_claim_review") is True
            ),
            "publication_eligible": sum(
                1 for item in admissibility if item.get("publication_eligible") is True
            ),
            "blocked": sum(1 for item in admissibility if item.get("status") == "blocked"),
            "blocking_reason_counts": dict(sorted(blocking_reason_counts.items())),
        },
    }


def main() -> int:
    args = parse_args()
    result = summarize(args.snapshot, args.evidence_review_manifest)
    if args.json:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(f"Snapshot: {result['snapshot_id']}")
        print(f"Git commit: {result['git_commit']}")
        print(f"Notes / units: {result['notes']} / {result['units']}")
        print(f"Direct references: {result['direct_reference_candidates']}")
        print(f"Bibliography references: {result['bibliography_reference_candidates']}")
        print(f"Providers: {result['provider_counts']}")
        print(f"Source types: {result['source_type_counts']}")
        print(f"Claim admissibility: {result['claim_admissibility']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
