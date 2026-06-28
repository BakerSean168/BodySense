"""Automatically ingest a local video into the normalized BodySense knowledge library."""

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
    for _env in [PROJECT_ROOT / ".env", PROJECT_ROOT.parent / ".env", PROJECT_ROOT.parent.parent / ".env"]:
        if _env.exists():
            load_dotenv(_env, override=False)
            break

from src.rag import (  # noqa: E402
    VideoIngestionPipeline,
    VideoIngestionRequest,
    get_knowledge_library,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("video_path", help="Path to the local source video")
    parser.add_argument("--problem-slug", required=True, help="Canonical problem slug")
    parser.add_argument(
        "--problem-display-name",
        required=True,
        help="Human readable problem name",
    )
    parser.add_argument("--author", required=True, help="Video author / expert name")
    parser.add_argument(
        "--source-title",
        help="Override the source title, defaults to the video stem",
    )
    parser.add_argument("--language", default="zh", help="Transcription language")
    parser.add_argument(
        "--transcript-provider",
        default=None,
        help="Transcript backend: whisper.cpp, funasr_sensevoice, or asr_api (default: ASR_PROVIDER env var, or whisper.cpp)",
    )
    parser.add_argument(
        "--transcript-model",
        help="Provider-specific transcript model name",
    )
    parser.add_argument(
        "--whisper-model",
        default="ggml-base.bin",
        help="Legacy whisper.cpp model filename; used when transcript provider is whisper.cpp",
    )
    parser.add_argument(
        "--force-transcribe",
        action="store_true",
        help="Re-run ASR even if artifacts exist",
    )
    parser.add_argument("--no-export-clips", action="store_true", help="Skip clip rendering")
    parser.add_argument(
        "--splitter-provider",
        default="heuristic",
        help="Splitter backend: heuristic (default) or llm",
    )
    parser.add_argument(
        "--ai-refine",
        action="store_true",
        help="Enable AI-assisted unit refinement (requires LLM provider)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Generate artifacts but do not write to DB",
    )
    parser.add_argument(
        "--overwrite-source",
        action="store_true",
        help="Replace an existing source with the same source_key",
    )
    return parser.parse_args()


async def main() -> int:
    args = parse_args()
    video_path = Path(args.video_path).resolve()
    pipeline = VideoIngestionPipeline()
    pack = await pipeline.ingest(
        VideoIngestionRequest(
            video_path=str(video_path),
            problem_slug=args.problem_slug,
            problem_display_name=args.problem_display_name,
            author=args.author,
            source_title=args.source_title or video_path.stem,
            language=args.language,
            transcript_provider=args.transcript_provider,
            transcript_model=args.transcript_model,
            whisper_model=args.whisper_model,
            force_transcribe=args.force_transcribe,
            export_clips=not args.no_export_clips,
            splitter_provider=args.splitter_provider,
            ai_refine=args.ai_refine,
        )
    )

    print(f"Source key: {pack.source.source_key}")
    print(f"Artifact dir: {pack.artifact_dir}")
    print(f"Transcript segments: {len(pack.transcript_segments)}")
    print(f"Knowledge units: {len(pack.units)}")
    print(f"Clips: {len(pack.clips)}")

    if args.dry_run:
        return 0

    library = get_knowledge_library()
    try:
        result = await library.ingest_generated_pack(pack, overwrite_source=args.overwrite_source)
    finally:
        await library.close()

    print(result)
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
