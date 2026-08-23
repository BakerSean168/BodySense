"""Validate and optionally ingest a Thought Forest health snapshot."""

from __future__ import annotations

import argparse
import asyncio
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

from src.rag.knowledge_library import KnowledgeLibrary  # noqa: E402
from src.rag.thought_forest_snapshot import (  # noqa: E402
    build_generated_packs,
    load_thought_forest_snapshot,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("snapshot", help="Path to bodysense.health.snapshot.v1 JSON")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and convert without DB writes",
    )
    parser.add_argument(
        "--overwrite-source",
        action="store_true",
        help="Replace existing Thought Forest sources with the same source_key",
    )
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    snapshot = load_thought_forest_snapshot(args.snapshot)
    packs = build_generated_packs(snapshot)
    unit_count = sum(len(pack.units) for pack in packs)
    print(f"Snapshot: {snapshot.snapshot_id}")
    print(f"Git commit: {snapshot.repository.git_commit}")
    print(f"Notes: {len(packs)}")
    print(f"Units: {unit_count}")
    print("Publication state: generated/unpublished")

    if args.dry_run:
        return 0

    library = KnowledgeLibrary()
    await library.initialize()
    try:
        for pack in packs:
            result = await library.ingest_generated_pack(
                pack, overwrite_source=args.overwrite_source
            )
            print(result)
    finally:
        await library.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
