"""Import a generated knowledge pack directly into the normalized BodySense knowledge library."""

from __future__ import annotations

import argparse
import asyncio
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

# Load .env before anything else reads os.getenv()
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

from src.rag import get_knowledge_library, load_generated_pack  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("generated_pack", help="Path to generated_pack.json")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Load the pack without writing to the database",
    )
    parser.add_argument(
        "--overwrite-source",
        action="store_true",
        help="Replace the existing source with the generated version",
    )
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    pack = load_generated_pack(args.generated_pack)

    print(f"Source key: {pack.source.source_key}")
    print(f"Units: {len(pack.units)}")
    print(f"Clips: {len(pack.clips)}")

    if args.dry_run:
        return 0

    library = get_knowledge_library()
    try:
        result = await library.ingest_generated_pack(
            pack,
            overwrite_source=args.overwrite_source,
        )
    finally:
        await library.close()

    print(result)
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
