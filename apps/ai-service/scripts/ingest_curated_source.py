"""Import a manually curated knowledge spec on top of an existing generated source pack."""

from __future__ import annotations

import argparse
import asyncio
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from src.rag import build_curated_pack, get_knowledge_library, load_generated_pack  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("generated_pack", help="Path to generated_pack.json")
    parser.add_argument("curated_spec", help="Path to the curated refinement JSON spec")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Build the curated pack and clips without writing to the database",
    )
    parser.add_argument(
        "--overwrite-source",
        action="store_true",
        help="Replace the existing source with the curated version",
    )
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    base_pack = load_generated_pack(args.generated_pack)
    curated_pack = build_curated_pack(base_pack, args.curated_spec)
    curated_json_path = Path(curated_pack.artifact_dir) / "curated_pack.json"
    curated_pack.write_json(curated_json_path)

    print(f"Source key: {curated_pack.source.source_key}")
    print(f"Curated units: {len(curated_pack.units)}")
    print(f"Curated clips: {len(curated_pack.clips)}")
    print(f"Curated artifact: {curated_json_path}")

    if args.dry_run:
        return 0

    library = get_knowledge_library()
    try:
        result = await library.ingest_generated_pack(
            curated_pack,
            overwrite_source=args.overwrite_source,
        )
    finally:
        await library.close()

    print(result)
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
