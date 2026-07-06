"""Video clip exporter — cuts demonstration clips from source video.

Only high-value unit types (self_check, exercise, warning) get exported as clips,
because these are the scenarios where users most benefit from "seeing the movement".
"""

from __future__ import annotations

import subprocess
from pathlib import Path

from .knowledge_pack import KnowledgeClipCandidate, KnowledgeUnitCandidate
from .splitter import CLIP_WORTHY_TYPES


def export_clips(
    video_path: Path,
    artifact_dir: Path,
    units: list[KnowledgeUnitCandidate],
    export_clips: bool = True,
) -> list[KnowledgeClipCandidate]:
    """Export video clips for high-value knowledge units.

    Processing rules:
        1. Pad start/end by 1.5s each (gives users context)
        2. Cut independent mp4 with ffmpeg
        3. Video codec: H.264 (best compatibility), Audio: AAC
        4. -movflags +faststart for progressive download (streaming-friendly)

    When export_clips=False (dry-run), clip metadata is still generated
    so you can preview which clips would be exported.
    """
    clips_dir = artifact_dir / "clips"
    clips_dir.mkdir(parents=True, exist_ok=True)

    clips: list[KnowledgeClipCandidate] = []
    for unit in units:
        if unit.unit_type not in CLIP_WORTHY_TYPES:
            continue

        start_sec = max(0.0, unit.source_start_sec - 1.5)
        end_sec = unit.source_end_sec + 1.5

        clip_key = f"{unit.unit_key}-clip"
        clip_path = clips_dir / f"{clip_key}.mp4"

        if export_clips:
            _cut_clip(video_path, clip_path, start_sec, end_sec)

        clips.append(
            KnowledgeClipCandidate(
                clip_key=clip_key,
                source_unit_key=unit.unit_key,
                clip_type=unit.unit_type,
                title=unit.title,
                start_sec=start_sec,
                end_sec=end_sec,
                file_path=str(clip_path),
                transcript_excerpt=unit.transcript_excerpt,
            )
        )
    return clips


def _cut_clip(
    video_path: Path,
    output_path: Path,
    start_sec: float,
    end_sec: float,
) -> None:
    """Cut a video clip using ffmpeg."""
    command = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel", "error",
        "-y",
        "-ss", str(start_sec),
        "-to", str(end_sec),
        "-i", str(video_path),
        "-c:v", "libx264",
        "-c:a", "aac",
        "-movflags", "+faststart",
        str(output_path),
    ]
    subprocess.run(command, check=True)
