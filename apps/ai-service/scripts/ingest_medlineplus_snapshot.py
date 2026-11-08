"""Ingest a reviewed MedlinePlus topic shard without publishing it."""

from __future__ import annotations

import argparse
import asyncio
import dataclasses
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

try:
    from dotenv import load_dotenv
except ModuleNotFoundError:
    load_dotenv = None

if load_dotenv:
    for _env in [
        PROJECT_ROOT / ".env",
        PROJECT_ROOT.parent / ".env",
        PROJECT_ROOT.parent.parent / ".env",
    ]:
        if _env.exists():
            load_dotenv(_env, override=False)
            break

from src.rag.claim_review import apply_claim_review, load_claim_review  # noqa: E402
from src.rag.external_evidence import (  # noqa: E402
    apply_external_evidence_review,
    build_claim_admissibility,
    load_external_evidence_review,
    resolve_external_reference,
)
from src.rag.knowledge_library import KnowledgeLibrary  # noqa: E402
from src.rag.medlineplus_topic import (  # noqa: E402
    build_external_reference,
    build_pack,
    build_units,
    load_rendering_plan,
    load_topic_xml,
    load_unit_spec,
    sha256_bytes,
)

DEFAULT_REVIEWED_BY = "adapter-self-review"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshot-dir", required=True)
    parser.add_argument("--topic", required=True)
    parser.add_argument("--shard", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--evidence-review-manifest", required=True)
    parser.add_argument("--claim-review-manifest", required=True)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--overwrite-source", action="store_true")
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    snapshot_dir = Path(args.snapshot_dir).expanduser().resolve()
    xml_path = snapshot_dir / "topics" / f"{args.topic}.xml"
    markdown_path = snapshot_dir / "topics" / f"{args.topic}.md"
    raw = xml_path.read_bytes()
    raw_sha256 = sha256_bytes(raw)
    record = load_topic_xml(xml_path, args.topic)
    plan = load_rendering_plan(snapshot_dir / "rendering" / f"{args.topic}.zh.json")
    spec = load_unit_spec(snapshot_dir / "units" / f"{args.topic}.{args.shard}.json")
    markdown = markdown_path.read_text(encoding="utf-8")

    units = build_units(
        record,
        plan,
        spec,
        markdown,
        content_commit=args.source_commit,
        raw_sha256=raw_sha256,
        markdown_path=f"topics/{args.topic}.md",
    )
    evidence_manifest = load_external_evidence_review(args.evidence_review_manifest)
    claim_manifest = load_claim_review(args.claim_review_manifest)
    if evidence_manifest.snapshot_git_commit != args.source_commit:
        raise SystemExit("evidence review snapshot_git_commit does not match --source-commit")
    if claim_manifest.snapshot_git_commit != args.source_commit:
        raise SystemExit("claim review snapshot_git_commit does not match --source-commit")
    if claim_manifest.external_evidence_review_id != evidence_manifest.review_id:
        raise SystemExit("claim review does not reference the supplied evidence review")

    reviewed_units = []
    for unit in units:
        claim_id = str(unit.metadata["claim_candidate"]["claim_id"])
        resolved = resolve_external_reference(
            build_external_reference(record, int(unit.source_start_sec))
        )
        reviewed_references = apply_external_evidence_review(
            claim_id=claim_id,
            claim_content_hash=str(unit.content_hash),
            resolved_references=[resolved],
            review_manifest=evidence_manifest,
        )
        admissibility = build_claim_admissibility(reviewed_references)
        admissibility, applied, review_status, lifecycle_status, quality_score = apply_claim_review(
            unit_key=unit.unit_key,
            claim_id=claim_id,
            claim_content_hash=str(unit.content_hash),
            claim_admissibility=admissibility,
            review_manifest=claim_manifest,
        )
        if applied is None:
            raise SystemExit(f"claim review produced no decision for {unit.unit_key}")
        reviewed_units.append(
            dataclasses.replace(
                unit,
                review_status=review_status,
                lifecycle_status=lifecycle_status,
                quality_score=quality_score,
                metadata={
                    **unit.metadata,
                    "external_evidence_candidates": reviewed_references,
                    "claim_admissibility": admissibility,
                    "claim_review": applied.model_dump(),
                    "review_role": DEFAULT_REVIEWED_BY,
                },
            )
        )

    pack = build_pack(
        record,
        reviewed_units,
        content_commit=args.source_commit,
        markdown_path=f"topics/{args.topic}.md",
        raw_sha256=raw_sha256,
    )
    print(f"source_key={pack.source.source_key}")
    print(f"units={len(pack.units)}")
    print(f"reviewed={sum(1 for unit in pack.units if unit.review_status == 'reviewed')}")
    if args.dry_run:
        return 0

    library = KnowledgeLibrary()
    await library.initialize()
    try:
        if args.overwrite_source:
            await library.assert_sources_overwritable([pack.source.source_key])
        print(await library.ingest_generated_pack(pack, overwrite_source=args.overwrite_source))
    finally:
        await library.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
