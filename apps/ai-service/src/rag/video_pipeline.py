"""
自动视频知识入库管道 (Automatic Video-to-Knowledge Ingestion Pipeline)

本文件是 Issue #13 "从视频到知识库" 自动链路的编排层。

整体流程：
    输入一个本地视频 (.mp4)
    → ffmpeg 抽取音频 (.wav)
    → ASR 语音转文字 (whisper.cpp / FunASR SenseVoice / ASR API)
    → 知识切分 (heuristic 关键词规则 / LLM 语义切分)
    → [可选] AI 精修 (逐单元润色+质量评分)
    → ffmpeg 导出动作演示片段 (clip_exporter.py)
    → 序列化为 generated_pack.json
    → 由 KnowledgeLibrary 写入数据库

设计原则：
    - 本文件只做编排和 IO，具体业务逻辑委托给子模块
    - 产出的是"自动底稿"，适合批量跑，但需要人工精修才能上线
    - 每个阶段的中间产物都落盘，便于复核和重跑
"""

from __future__ import annotations

import logging
import subprocess
from dataclasses import dataclass
from pathlib import Path

from .asr import get_asr_provider
from .clip_exporter import export_clips
from .knowledge_pack import (
    GeneratedKnowledgePack,
    SourceVideoMetadata,
    TranscriptSegment,
    slugify,
)
from .splitter import get_splitter

logger = logging.getLogger(__name__)

# Default data root: apps/ai-service/data/
_DEFAULT_DATA_ROOT = Path(__file__).resolve().parents[2] / "data"


@dataclass(frozen=True)
class VideoIngestionRequest:
    """Input parameters for a single automatic video ingestion.

    Typical usage (from ingest_video_source.py):
        request = VideoIngestionRequest(
            video_path="C:/Users/baker/Videos/凯圣王/头前移.mp4",
            problem_slug="forward-head-posture",
            problem_display_name="头前移",
            author="凯圣王",
            source_title="头前移完整矫正指南",
        )
        pipeline = VideoIngestionPipeline()
        pack = await pipeline.ingest(request)
    """

    video_path: str
    problem_slug: str
    problem_display_name: str
    author: str
    source_title: str
    language: str = "zh"
    transcript_provider: str | None = None
    transcript_model: str | None = None
    whisper_model: str = "ggml-base.bin"
    force_transcribe: bool = False
    export_clips: bool = True
    splitter_provider: str = "heuristic"  # "heuristic" | "llm"
    ai_refine: bool = False  # 是否启用 AI 精修


class VideoIngestionPipeline:
    """Automatic video-to-knowledge ingestion pipeline (orchestrator).

    Responsibilities:
        1. Accept a local video + metadata
        2. Extract audio → ASR transcribe → knowledge split → [AI refine] → export clips → pack
        3. Output generated_pack.json for downstream ingestion

    Directory layout:
        data/
          knowledge_sources/
            {source_key}/
              audio.wav
              transcript.raw.jsonl
              transcript.txt
              generated_pack.json
              clips/
          .cache/
            whisper/
            funasr_runtime/
            funasr_models/
    """

    def __init__(self, data_root: str | Path | None = None):
        self.data_root = Path(data_root or _DEFAULT_DATA_ROOT).resolve()
        self.sources_root = self.data_root / "knowledge_sources"

    async def ingest(self, request: VideoIngestionRequest) -> GeneratedKnowledgePack:
        """Execute the end-to-end ingestion pipeline.

        Returns:
            GeneratedKnowledgePack, also written to disk as generated_pack.json.
        """
        # --- Step 1: Validate video exists ---
        video_path = Path(request.video_path).resolve()
        if not video_path.exists():
            raise FileNotFoundError(f"Video not found: {video_path}")

        # --- Step 2: Generate unique source_key ---
        source_key = slugify(f"{request.author}-{request.problem_slug}-{video_path.stem}")
        artifact_dir = self.sources_root / source_key
        artifact_dir.mkdir(parents=True, exist_ok=True)

        # --- Step 3: ASR transcription ---
        transcript_segments = await self._transcribe(
            video_path=video_path,
            artifact_dir=artifact_dir,
            request=request,
        )

        # --- Step 4: Knowledge splitting ---
        splitter = get_splitter(request.splitter_provider)
        units = await splitter.split(
            transcript_segments=transcript_segments,
            problem_slug=request.problem_slug,
            problem_display_name=request.problem_display_name,
        )

        # --- Step 5: Export video clips ---
        clips = export_clips(
            video_path=video_path,
            artifact_dir=artifact_dir,
            units=units,
            export_clips=request.export_clips,
        )

        # --- Step 6: Assemble knowledge pack ---
        pack = GeneratedKnowledgePack(
            source=SourceVideoMetadata(
                source_key=source_key,
                source_type="video",
                title=request.source_title,
                author=request.author,
                problem_slug=request.problem_slug,
                problem_display_name=request.problem_display_name,
                original_file_path=str(video_path),
                language=request.language,
                duration_sec=_probe_duration(video_path),
                transcript_provider=request.transcript_provider,
                transcript_model=request.transcript_model or request.whisper_model,
                transcript_file_path=str(artifact_dir / "transcript.txt"),
                metadata={
                    "video_stem": video_path.stem,
                    "artifact_dir": str(artifact_dir),
                    "splitter_provider": request.splitter_provider,
                },
            ),
            artifact_dir=str(artifact_dir),
            transcript_segments=transcript_segments,
            units=units,
            clips=clips,
        )

        # --- Step 7: Optional AI refinement ---
        if request.ai_refine:
            from .ai_curator import AICurator

            curator = AICurator()
            pack = await curator.refine_pack(pack)

        # --- Step 8: Write to disk ---
        pack.write_json(artifact_dir / "generated_pack.json")
        return pack

    async def _transcribe(
        self,
        video_path: Path,
        artifact_dir: Path,
        request: VideoIngestionRequest,
    ) -> list[TranscriptSegment]:
        """Extract audio and run ASR transcription.

        Idempotent: if transcript already exists and force=False, reads from disk.
        """
        transcript_jsonl = artifact_dir / "transcript.raw.jsonl"
        transcript_text = artifact_dir / "transcript.txt"

        if request.force_transcribe or not transcript_jsonl.exists():
            # Extract audio (mono, 16kHz WAV — standard for ASR models)
            audio_path = artifact_dir / "audio.wav"
            _extract_audio(video_path, audio_path)

            # Create ASR provider and transcribe
            provider_name = (
                request.transcript_provider.strip().lower()
                if request.transcript_provider
                else None
            )
            provider = get_asr_provider(provider=provider_name, data_root=self.data_root)

            # For local providers, pass model name if specified
            if hasattr(provider, "model_name") and request.transcript_model:
                provider.model_name = request.transcript_model

            segments = await provider.transcribe(audio_path, language=request.language)
        else:
            # Read cached transcript
            from .asr.whisper_cpp import _parse_transcript_jsonl

            segments = _parse_transcript_jsonl(transcript_jsonl)

        # Generate human-readable transcript
        _render_transcript_text(segments, transcript_text)

        return segments


# ---------------------------------------------------------------------------
# Module-level utilities
# ---------------------------------------------------------------------------


_SUBPROCESS_TIMEOUT = 300  # 5 minutes


def _probe_duration(video_path: Path) -> float | None:
    """Probe video duration using ffprobe."""
    command = [
        "ffprobe",
        "-v", "error",
        "-show_entries", "format=duration",
        "-of", "default=noprint_wrappers=1:nokey=1",
        str(video_path),
    ]
    result = subprocess.run(
        command, check=True, capture_output=True, text=True,
        timeout=_SUBPROCESS_TIMEOUT,
    )
    value = result.stdout.strip()
    return float(value) if value else None


def _extract_audio(video_path: Path, audio_path: Path) -> None:
    """Extract audio from video using ffmpeg (mono, 16kHz WAV)."""
    command = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel", "error",
        "-y",
        "-i", str(video_path),
        "-vn",
        "-ac", "1",
        "-ar", "16000",
        str(audio_path),
    ]
    subprocess.run(command, check=True, timeout=_SUBPROCESS_TIMEOUT)


def _render_transcript_text(segments: list[TranscriptSegment], output_path: Path) -> None:
    """Generate human-readable transcript text file."""
    lines = [f"[{segment.timestamp}] {segment.text}" for segment in segments]
    output_path.write_text("\n".join(lines), encoding="utf-8")
