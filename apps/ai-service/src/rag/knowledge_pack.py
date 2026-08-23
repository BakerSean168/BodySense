"""Domain models for automatically generated video-derived knowledge."""

from __future__ import annotations

import json
import re
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any


def slugify(value: str) -> str:
    """Convert a value into a filesystem and key-safe slug."""
    normalized = re.sub(r"[^\w\s-]", " ", value.strip().lower())
    normalized = re.sub(r"[\s_]+", "-", normalized)
    normalized = re.sub(r"-{2,}", "-", normalized)
    return normalized.strip("-") or "item"


def format_seconds(seconds: float) -> str:
    """Render a timestamp in HH:MM:SS or MM:SS format."""
    total_seconds = max(0, int(round(seconds)))
    hours, remainder = divmod(total_seconds, 3600)
    minutes, secs = divmod(remainder, 60)
    if hours:
        return f"{hours:02d}:{minutes:02d}:{secs:02d}"
    return f"{minutes:02d}:{secs:02d}"


def format_timestamp_range(start_sec: float | None, end_sec: float | None) -> str | None:
    """Render a source timestamp range suitable for the database."""
    if start_sec is None and end_sec is None:
        return None
    if start_sec is not None and end_sec is not None:
        return f"{format_seconds(start_sec)}-{format_seconds(end_sec)}"
    if start_sec is not None:
        return format_seconds(start_sec)
    return format_seconds(end_sec or 0)


@dataclass(frozen=True)
class TranscriptSegment:
    """A transcript segment produced by ASR."""

    segment_index: int
    start_sec: float
    end_sec: float
    text: str
    confidence: float | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    @property
    def timestamp(self) -> str:
        return format_timestamp_range(self.start_sec, self.end_sec) or "00:00"


@dataclass(frozen=True)
class KnowledgeUnitCandidate:
    """A searchable knowledge unit derived from transcript segments."""

    unit_key: str
    problem_slug: str
    problem_display_name: str
    category: str
    unit_type: str
    title: str
    summary: str
    body_markdown: str
    source_start_sec: float
    source_end_sec: float
    evidence_segment_indices: list[int]
    tags: list[str] = field(default_factory=list)
    transcript_excerpt: str = ""
    review_status: str = "generated"
    lifecycle_status: str = "generated"
    quality_score: float = 0.0
    content_hash: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.source_start_sec, self.source_end_sec) or "00:00"


@dataclass(frozen=True)
class KnowledgeClipCandidate:
    """A clip exported from the source video for demonstration or explanation."""

    clip_key: str
    source_unit_key: str
    clip_type: str
    title: str
    start_sec: float
    end_sec: float
    file_path: str
    transcript_excerpt: str
    notes: str | None = None

    @property
    def duration_sec(self) -> float:
        return max(0.0, self.end_sec - self.start_sec)

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.start_sec, self.end_sec) or "00:00"


@dataclass(frozen=True)
class SourceVideoMetadata:
    """Metadata about the original source video."""

    source_key: str
    source_type: str
    title: str
    author: str
    problem_slug: str
    problem_display_name: str
    original_file_path: str
    language: str = "zh"
    duration_sec: float | None = None
    transcript_provider: str = "whisper.cpp"
    transcript_model: str | None = None
    transcript_file_path: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)


@dataclass(frozen=True)
class GeneratedKnowledgePack:
    """A fully generated pack ready for ingestion into the knowledge library."""

    source: SourceVideoMetadata
    artifact_dir: str
    transcript_segments: list[TranscriptSegment]
    units: list[KnowledgeUnitCandidate]
    clips: list[KnowledgeClipCandidate]

    def to_dict(self) -> dict[str, Any]:
        """Convert the pack to a JSON-serializable dictionary."""
        return {
            "source": asdict(self.source),
            "artifact_dir": self.artifact_dir,
            "transcript_segments": [asdict(segment) for segment in self.transcript_segments],
            "units": [asdict(unit) for unit in self.units],
            "clips": [asdict(clip) for clip in self.clips],
        }

    def write_json(self, path: str | Path) -> Path:
        """Persist the generated pack for inspection and manual refinement."""
        resolved = Path(path).resolve()
        resolved.parent.mkdir(parents=True, exist_ok=True)
        resolved.write_text(
            json.dumps(self.to_dict(), ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        return resolved
