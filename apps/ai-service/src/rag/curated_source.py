"""Utilities for refining an auto-generated video pack into curated knowledge."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any

from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeClipCandidate,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
)


def load_generated_pack(path: str | Path) -> GeneratedKnowledgePack:
    """Load a generated pack JSON artifact into dataclasses."""
    resolved = Path(path).resolve()
    payload = json.loads(resolved.read_text(encoding="utf-8"))

    source = payload["source"]
    transcript_segments = [
        TranscriptSegment(**segment)
        for segment in payload["transcript_segments"]
    ]
    units = [
        KnowledgeUnitCandidate(**unit)
        for unit in payload["units"]
    ]
    clips = [
        KnowledgeClipCandidate(**clip)
        for clip in payload["clips"]
    ]

    return GeneratedKnowledgePack(
        source=SourceVideoMetadata(**source),
        artifact_dir=payload["artifact_dir"],
        transcript_segments=transcript_segments,
        units=units,
        clips=clips,
    )


def collect_evidence_segment_indices(
    transcript_segments: list[TranscriptSegment],
    start_sec: float,
    end_sec: float,
) -> list[int]:
    """Collect segment indices overlapping the requested time range."""
    indices: list[int] = []
    for segment in transcript_segments:
        overlaps = segment.end_sec > start_sec and segment.start_sec < end_sec
        if overlaps:
            indices.append(segment.segment_index)
    return indices


def collect_transcript_excerpt(
    transcript_segments: list[TranscriptSegment],
    segment_indices: list[int],
) -> str:
    """Join transcript text for the provided segment indices."""
    by_index = {segment.segment_index: segment.text for segment in transcript_segments}
    parts = [by_index[index] for index in segment_indices if index in by_index]
    return " ".join(parts).strip()


def build_curated_pack(
    base_pack: GeneratedKnowledgePack,
    spec_path: str | Path,
) -> GeneratedKnowledgePack:
    """Build a curated pack from a generated base pack and a refinement spec."""
    resolved_spec_path = Path(spec_path).resolve()
    spec = json.loads(resolved_spec_path.read_text(encoding="utf-8"))

    base_source = base_pack.source
    source_key = spec.get("source_key", base_source.source_key)
    curated_units: list[KnowledgeUnitCandidate] = []
    curated_clips: list[KnowledgeClipCandidate] = []

    for unit_spec in spec["units"]:
        start_sec = float(unit_spec["source_start_sec"])
        end_sec = float(unit_spec["source_end_sec"])
        evidence_indices = unit_spec.get("evidence_segment_indices")
        if not evidence_indices:
            evidence_indices = collect_evidence_segment_indices(
                base_pack.transcript_segments,
                start_sec,
                end_sec,
            )

        transcript_excerpt = unit_spec.get("transcript_excerpt")
        if not transcript_excerpt:
            transcript_excerpt = collect_transcript_excerpt(
                base_pack.transcript_segments,
                evidence_indices,
            )

        curated_units.append(
            KnowledgeUnitCandidate(
                unit_key=unit_spec["unit_key"],
                problem_slug=spec["problem_slug"],
                problem_display_name=spec["problem_display_name"],
                category=unit_spec["category"],
                unit_type=unit_spec["unit_type"],
                title=unit_spec["title"],
                summary=unit_spec["summary"],
                body_markdown=unit_spec["body_markdown"],
                source_start_sec=start_sec,
                source_end_sec=end_sec,
                evidence_segment_indices=evidence_indices,
                tags=unit_spec.get("tags", []),
                transcript_excerpt=transcript_excerpt,
                review_status=unit_spec.get("review_status", "curated"),
            )
        )

    clips_dir = Path(base_pack.artifact_dir) / "clips"
    clips_dir.mkdir(parents=True, exist_ok=True)
    video_path = Path(base_source.original_file_path)

    for clip_spec in spec.get("clips", []):
        start_sec = float(clip_spec["start_sec"])
        end_sec = float(clip_spec["end_sec"])
        segment_indices = collect_evidence_segment_indices(
            base_pack.transcript_segments,
            start_sec,
            end_sec,
        )
        transcript_excerpt = collect_transcript_excerpt(
            base_pack.transcript_segments,
            segment_indices,
        )
        clip_key = clip_spec["clip_key"]
        clip_path = clips_dir / f"{clip_key}.mp4"

        export_clip(
            video_path=video_path,
            output_path=clip_path,
            start_sec=start_sec,
            end_sec=end_sec,
        )

        curated_clips.append(
            KnowledgeClipCandidate(
                clip_key=clip_key,
                source_unit_key=clip_spec["source_unit_key"],
                clip_type=clip_spec["clip_type"],
                title=clip_spec["title"],
                start_sec=start_sec,
                end_sec=end_sec,
                file_path=str(clip_path),
                transcript_excerpt=transcript_excerpt,
                notes=clip_spec.get("notes"),
            )
        )

    metadata: dict[str, Any] = dict(base_source.metadata)
    metadata.update(
        {
            "curation_status": "curated",
            "curated_spec_path": str(resolved_spec_path),
        }
    )

    source = SourceVideoMetadata(
        source_key=source_key,
        source_type=base_source.source_type,
        title=spec.get("source_title", base_source.title),
        author=spec.get("author", base_source.author),
        problem_slug=spec["problem_slug"],
        problem_display_name=spec["problem_display_name"],
        original_file_path=base_source.original_file_path,
        language=base_source.language,
        duration_sec=base_source.duration_sec,
        transcript_provider=base_source.transcript_provider,
        transcript_model=base_source.transcript_model,
        transcript_file_path=base_source.transcript_file_path,
        metadata=metadata,
    )

    return GeneratedKnowledgePack(
        source=source,
        artifact_dir=base_pack.artifact_dir,
        transcript_segments=base_pack.transcript_segments,
        units=curated_units,
        clips=curated_clips,
    )


def export_clip(
    video_path: Path,
    output_path: Path,
    start_sec: float,
    end_sec: float,
) -> None:
    """Export a clip from the source video."""
    command = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel",
        "error",
        "-y",
        "-ss",
        str(start_sec),
        "-to",
        str(end_sec),
        "-i",
        str(video_path),
        "-c:v",
        "libx264",
        "-c:a",
        "aac",
        "-movflags",
        "+faststart",
        str(output_path),
    ]
    subprocess.run(command, check=True)
