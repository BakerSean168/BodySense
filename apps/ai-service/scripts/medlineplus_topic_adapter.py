"""Render and materialize MedlinePlus topic snapshots into reviewed knowledge artifacts."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from src.rag.claim_review import (  # noqa: E402  # noqa: E402
    CLAIM_REVIEW_SCHEMA_VERSION,
    ReviewedKnowledgeUnit,
    apply_claim_review,
    build_reviewed_snapshot,
    load_claim_review,
)
from src.rag.external_evidence import (  # noqa: E402
    EXTERNAL_EVIDENCE_REVIEW_SCHEMA_VERSION,
    apply_external_evidence_review,
    build_claim_admissibility,
    load_external_evidence_review,
    resolve_external_reference,
)
from src.rag.medlineplus_topic import (  # noqa: E402
    SOURCE_TYPE,
    build_external_reference,
    build_units,
    load_rendering_plan,
    load_topic_xml,
    load_unit_spec,
    render_markdown,
    sha256_bytes,
    sha256_text,
)

DEFAULT_REVIEWED_BY = "adapter-self-review"
DEFAULT_REVIEWED_AT = "2026-11-08T21:30:00Z"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    render = subparsers.add_parser("render", help="XML -> deterministic Markdown")
    render.add_argument("--snapshot-dir", required=True)
    render.add_argument("--topic", required=True)

    materialize = subparsers.add_parser("materialize", help="produce reviewed-pack artifacts")
    materialize.add_argument("--snapshot-dir", required=True)
    materialize.add_argument("--topic", required=True)
    materialize.add_argument("--shard", required=True)
    materialize.add_argument("--source-commit", required=True)
    materialize.add_argument("--output-dir", required=True)
    materialize.add_argument("--reviewed-by", default=DEFAULT_REVIEWED_BY)
    materialize.add_argument("--reviewed-at", default=DEFAULT_REVIEWED_AT)
    return parser.parse_args()


def _paths(snapshot_dir: Path, topic: str) -> dict[str, Path]:
    return {
        "xml": snapshot_dir / "topics" / f"{topic}.xml",
        "markdown": snapshot_dir / "topics" / f"{topic}.md",
        "rendering": snapshot_dir / "rendering" / f"{topic}.zh.json",
        "capture": snapshot_dir / "manifests" / f"{topic}.capture.json",
        "snapshot": snapshot_dir / "manifests" / f"{topic}.snapshot.json",
    }


def render(args: argparse.Namespace) -> int:
    snapshot_dir = Path(args.snapshot_dir).expanduser().resolve()
    paths = _paths(snapshot_dir, args.topic)
    raw = paths["xml"].read_bytes()
    raw_sha256 = sha256_bytes(raw)
    record = load_topic_xml(paths["xml"], args.topic)
    plan = load_rendering_plan(paths["rendering"])
    markdown = render_markdown(
        record, plan, raw_sha256=raw_sha256, xml_path=f"topics/{args.topic}.xml"
    )
    paths["markdown"].write_text(markdown, encoding="utf-8")
    print(f"topic={record.title}")
    print(f"raw_sha256={raw_sha256}")
    print(f"markdown_sha256={sha256_text(markdown)}")
    print(f"markdown_lines={markdown.count(chr(10))}")
    print(f"markdown={paths['markdown']}")
    return 0


def _write_json(path: Path, payload: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def materialize(args: argparse.Namespace) -> int:
    snapshot_dir = Path(args.snapshot_dir).expanduser().resolve()
    paths = _paths(snapshot_dir, args.topic)
    raw = paths["xml"].read_bytes()
    raw_sha256 = sha256_bytes(raw)
    record = load_topic_xml(paths["xml"], args.topic)
    plan = load_rendering_plan(paths["rendering"])
    spec = load_unit_spec(snapshot_dir / "units" / f"{args.topic}.{args.shard}.json")

    expected_markdown = render_markdown(
        record, plan, raw_sha256=raw_sha256, xml_path=f"topics/{args.topic}.xml"
    )
    committed_markdown = paths["markdown"].read_text(encoding="utf-8")
    if committed_markdown != expected_markdown:
        raise SystemExit(
            "committed Markdown is not the deterministic rendering of the captured XML; "
            "re-run render and re-commit before materializing"
        )

    units = build_units(
        record,
        plan,
        spec,
        committed_markdown,
        content_commit=args.source_commit,
        raw_sha256=raw_sha256,
        markdown_path=f"topics/{args.topic}.md",
    )

    evidence_id = f"medlineplus-knee-{args.shard}-evidence-review-2026-11-08"
    claim_review_id = f"medlineplus-knee-{args.shard}-claim-review-2026-11-08"
    sources = [
        {
            "canonical_key": resolve_external_reference(build_external_reference(record, 1))[
                "canonical_key"
            ],
            "canonical_url": record.canonical_url,
            "authority_tier": "B",
            "evidence_level": "governmental_health_topic_summary",
            "license_status": "verified_reuse",
            "review_status": "reviewed",
            "reviewed_by": args.reviewed_by,
            "review_basis": (
                "MedlinePlus states that summaries on health topic pages are public-domain "
                "U.S. federal government work reusable with attribution, and the captured "
                "topic page carries no third-party copyright label. The topic page is the "
                "direct supporting source for every claim in this shard."
            ),
        }
    ]
    relations = [
        {
            "claim_id": unit.metadata["claim_candidate"]["claim_id"],
            "claim_content_hash": str(unit.content_hash),
            "canonical_key": sources[0]["canonical_key"],
            "support_status": "reviewed_support",
            "support_scope": "direct",
            "review_status": "reviewed",
        }
        for unit in units
    ]
    evidence_payload = {
        "schema_version": EXTERNAL_EVIDENCE_REVIEW_SCHEMA_VERSION,
        "review_id": evidence_id,
        "snapshot_git_commit": args.source_commit,
        "reviewed_at": args.reviewed_at,
        "sources": sources,
        "relations": relations,
    }
    claims_payload = {
        "schema_version": CLAIM_REVIEW_SCHEMA_VERSION,
        "review_id": claim_review_id,
        "snapshot_git_commit": args.source_commit,
        "external_evidence_review_id": evidence_id,
        "reviewed_at": args.reviewed_at,
        "decisions": [
            {
                "unit_key": unit.unit_key,
                "claim_id": unit.metadata["claim_candidate"]["claim_id"],
                "claim_content_hash": str(unit.content_hash),
                "decision": "approved",
                "review_status": "reviewed",
                "reviewed_by": args.reviewed_by,
                "review_basis": str((spec.get("review_basis") or {}).get(unit.unit_key, "")),
                "quality_score": float((spec.get("quality_scores") or {}).get(unit.unit_key, 0.0)),
                "certainty": str((spec.get("certainty") or {}).get(unit.unit_key, "moderate")),
                "population": "general",
            }
            for unit in units
        ],
    }

    output_dir = Path(args.output_dir).expanduser().resolve()
    prefix = f"medlineplus-knee-{args.shard}"
    _write_json(output_dir / f"{prefix}-evidence-review.json", evidence_payload)
    _write_json(output_dir / f"{prefix}-claim-review.json", claims_payload)

    evidence_manifest = load_external_evidence_review(output_dir / f"{prefix}-evidence-review.json")
    claim_manifest = load_claim_review(output_dir / f"{prefix}-claim-review.json")
    reviewed_units: list[ReviewedKnowledgeUnit] = []
    for unit in units:
        reference = build_external_reference(record, int(unit.source_start_sec))
        resolved = resolve_external_reference(reference)
        reviewed_references = apply_external_evidence_review(
            claim_id=unit.metadata["claim_candidate"]["claim_id"],
            claim_content_hash=str(unit.content_hash),
            resolved_references=[resolved],
            review_manifest=evidence_manifest,
        )
        admissibility = build_claim_admissibility(reviewed_references)
        admissibility, applied, review_status, lifecycle_status, quality_score = apply_claim_review(
            unit_key=unit.unit_key,
            claim_id=unit.metadata["claim_candidate"]["claim_id"],
            claim_content_hash=str(unit.content_hash),
            claim_admissibility=admissibility,
            review_manifest=claim_manifest,
        )
        if applied is None:
            raise SystemExit(f"claim review produced no decision for {unit.unit_key}")
        reviewed_units.append(
            ReviewedKnowledgeUnit(
                unit_key=unit.unit_key,
                claim_id=unit.metadata["claim_candidate"]["claim_id"],
                claim_content_hash=str(unit.content_hash),
                review_status=review_status,
                lifecycle_status=lifecycle_status,
                quality_score=quality_score,
                publication_eligible=admissibility.get("publication_eligible") is True,
                source_locator=dict(unit.metadata["source_locator"]),
                claim_review=applied.model_dump(),
            )
        )

    reviewed = build_reviewed_snapshot(
        source_snapshot_id=str(units[0].metadata["snapshot_id"]),
        source_git_commit=args.source_commit,
        external_evidence_review_id=evidence_manifest.review_id,
        claim_review_id=claim_manifest.review_id,
        units=reviewed_units,
    )
    _write_json(output_dir / f"{prefix}-reviewed-knowledge.json", reviewed.model_dump())

    snapshot_payload = {
        "schema_version": "bodysense.source-snapshot.medlineplus.v1",
        "snapshot_id": str(units[0].metadata["snapshot_id"]),
        "content_commit": args.source_commit,
        "source": {
            "source_key": f"medlineplus:health-topic:{args.topic}",
            "source_type": SOURCE_TYPE,
            "title": record.title,
            "author": "MedlinePlus / U.S. National Library of Medicine",
            "problem_slug": str(units[0].problem_slug),
            "problem_display_name": str(units[0].problem_display_name),
            "language": "zh",
            "original_file_path": f"topics/{args.topic}.xml",
            "canonical_url": record.canonical_url,
            "source_version": f"topic-{record.topic_id}",
            "content_hash": raw_sha256,
            "license_status": "public_domain",
            "provenance": {
                "publisher": "MedlinePlus (NLM)",
                "retrieved_at": "2026-11-08T21:01:55Z",
                "raw_xml_sha256": raw_sha256,
                "markdown_sha256": sha256_text(committed_markdown),
                "attribution": "Source: MedlinePlus, National Library of Medicine",
                "license_basis": (
                    "https://medlineplus.gov/about/using/usingcontent/ states that summaries "
                    "on health topic pages are public domain with attribution; the captured "
                    "topic page carries no third-party copyright label."
                ),
                "translation": {
                    "from": "en",
                    "to": "zh",
                    "method": str(plan.get("translation_method") or ""),
                },
            },
        },
        "documents": [
            {
                "path": f"topics/{args.topic}.md",
                "sha256": sha256_text(committed_markdown),
                "line_count": committed_markdown.count("\n"),
                "sections": [
                    {
                        "heading_path": unit.metadata["source_locator"]["heading_path"],
                        "line_start": unit.metadata["source_locator"]["line_start"],
                        "line_end": unit.metadata["source_locator"]["line_end"],
                    }
                    for unit in units
                ],
            }
        ],
    }
    _write_json(paths["snapshot"], snapshot_payload)

    print(f"shard={args.shard} units={len(units)}")
    print(f"source_snapshot_id={units[0].metadata['snapshot_id']}")
    print(f"reviewed_snapshot_id={reviewed.reviewed_snapshot_id}")
    print(f"evidence_review={output_dir / f'{prefix}-evidence-review.json'}")
    print(f"claim_review={output_dir / f'{prefix}-claim-review.json'}")
    print(f"reviewed_snapshot={output_dir / f'{prefix}-reviewed-knowledge.json'}")
    print(f"snapshot_manifest={paths['snapshot']}")
    return 0


def main() -> int:
    args = parse_args()
    if args.command == "render":
        return render(args)
    return materialize(args)


if __name__ == "__main__":
    raise SystemExit(main())
