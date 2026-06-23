"""Automatic video-to-knowledge ingestion pipeline."""

from __future__ import annotations

import json
import os
import re
import subprocess
import urllib.request
import zipfile
from collections import Counter
from dataclasses import dataclass
from pathlib import Path

from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeClipCandidate,
    KnowledgeUnitCandidate,
    SourceVideoMetadata,
    TranscriptSegment,
    slugify,
)

WHISPER_MODEL_URLS = {
    "ggml-tiny.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
    "ggml-base.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
    "ggml-small.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
}
FUNASR_RUNTIME_VERSION = "runtime-llamacpp-v0.1.1"
FUNASR_RUNTIME_URL = (
    "https://github.com/modelscope/FunASR/releases/download/"
    f"{FUNASR_RUNTIME_VERSION}/funasr-llamacpp-windows-x64.zip"
)
FUNASR_MODEL_CONFIGS = {
    "sensevoice-small-q8.gguf": {
        "binary_name": "llama-funasr-sensevoice.exe",
        "repo_id": "FunAudioLLM/SenseVoiceSmall-GGUF",
        "file_name": "sensevoice-small-q8.gguf",
    }
}
TRANSCRIPT_PROVIDERS = {"whisper.cpp", "funasr_sensevoice"}
CLIP_WORTHY_TYPES = {"self_check", "exercise", "warning"}


@dataclass(frozen=True)
class VideoIngestionRequest:
    """Input parameters for an automatic video ingestion run."""

    video_path: str
    problem_slug: str
    problem_display_name: str
    author: str
    source_title: str
    language: str = "zh"
    transcript_provider: str = "whisper.cpp"
    transcript_model: str | None = None
    whisper_model: str = "ggml-base.bin"
    force_transcribe: bool = False
    export_clips: bool = True


class VideoIngestionPipeline:
    """Transcribe a video, derive knowledge units, and export helpful clips."""

    def __init__(self, data_root: str | Path | None = None):
        self.data_root = Path(data_root or Path(__file__).resolve().parents[2] / "data").resolve()
        self.sources_root = self.data_root / "knowledge_sources"
        self.whisper_root = self.data_root / ".cache" / "whisper"
        self.funasr_runtime_root = self.data_root / ".cache" / "funasr_runtime"
        self.funasr_model_root = self.data_root / ".cache" / "funasr_models"

    def ingest(self, request: VideoIngestionRequest) -> GeneratedKnowledgePack:
        """Run the end-to-end local ingestion pipeline for one video."""
        video_path = Path(request.video_path).resolve()
        if not video_path.exists():
            raise FileNotFoundError(f"Video not found: {video_path}")

        source_key = slugify(f"{request.author}-{request.problem_slug}-{video_path.stem}")
        artifact_dir = self.sources_root / source_key
        artifact_dir.mkdir(parents=True, exist_ok=True)

        transcript_segments, transcript_path = self._transcribe_video(
            video_path=video_path,
            artifact_dir=artifact_dir,
            transcript_provider=request.transcript_provider,
            transcript_model=request.transcript_model or request.whisper_model,
            whisper_model=request.whisper_model,
            language=request.language,
            force=request.force_transcribe,
        )
        units = self._build_units(
            transcript_segments=transcript_segments,
            problem_slug=request.problem_slug,
            problem_display_name=request.problem_display_name,
        )
        clips = self._export_clips(
            video_path=video_path,
            artifact_dir=artifact_dir,
            units=units,
            export_clips=request.export_clips,
        )

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
                duration_sec=self._probe_duration(video_path),
                transcript_provider=request.transcript_provider,
                transcript_model=request.transcript_model or request.whisper_model,
                transcript_file_path=str(transcript_path),
                metadata={
                    "video_stem": video_path.stem,
                    "artifact_dir": str(artifact_dir),
                },
            ),
            artifact_dir=str(artifact_dir),
            transcript_segments=transcript_segments,
            units=units,
            clips=clips,
        )
        pack.write_json(artifact_dir / "generated_pack.json")
        return pack

    def _probe_duration(self, video_path: Path) -> float | None:
        command = [
            "ffprobe",
            "-v",
            "error",
            "-show_entries",
            "format=duration",
            "-of",
            "default=noprint_wrappers=1:nokey=1",
            str(video_path),
        ]
        result = subprocess.run(command, check=True, capture_output=True, text=True)
        value = result.stdout.strip()
        return float(value) if value else None

    def _transcribe_video(
        self,
        video_path: Path,
        artifact_dir: Path,
        transcript_provider: str,
        transcript_model: str | None,
        whisper_model: str,
        language: str,
        force: bool,
    ) -> tuple[list[TranscriptSegment], Path]:
        transcript_jsonl = artifact_dir / "transcript.raw.jsonl"
        transcript_text = artifact_dir / "transcript.txt"

        if force or not transcript_jsonl.exists():
            audio_path = artifact_dir / "audio.wav"
            self._extract_audio(video_path, audio_path)
            provider = transcript_provider.strip().lower()
            if provider not in TRANSCRIPT_PROVIDERS:
                supported = ", ".join(sorted(TRANSCRIPT_PROVIDERS))
                raise ValueError(
                    "Unsupported transcript provider "
                    f"'{transcript_provider}'. Supported: {supported}"
                )

            if provider == "whisper.cpp":
                model_name = transcript_model or whisper_model
                model_path = self._ensure_whisper_model(model_name)
                self._run_whisper(
                    audio_path=audio_path,
                    output_path=transcript_jsonl,
                    model_path=model_path,
                    language=language,
                )
            else:
                model_name = transcript_model or "sensevoice-small-q8.gguf"
                self._run_funasr_sensevoice(
                    audio_path=audio_path,
                    output_path=transcript_jsonl,
                    model_name=model_name,
                )

        segments = self._parse_transcript_jsonl(transcript_jsonl)
        transcript_text.write_text(self._render_transcript_text(segments), encoding="utf-8")
        return segments, transcript_text

    def _ensure_whisper_model(self, model_name: str) -> Path:
        if model_name not in WHISPER_MODEL_URLS:
            supported = ", ".join(sorted(WHISPER_MODEL_URLS))
            raise ValueError(f"Unsupported whisper model '{model_name}'. Supported: {supported}")

        self.whisper_root.mkdir(parents=True, exist_ok=True)
        model_path = self.whisper_root / model_name
        if not model_path.exists():
            urllib.request.urlretrieve(WHISPER_MODEL_URLS[model_name], model_path)
        return model_path

    def _extract_audio(self, video_path: Path, audio_path: Path) -> None:
        command = [
            "ffmpeg",
            "-hide_banner",
            "-loglevel",
            "error",
            "-y",
            "-i",
            str(video_path),
            "-vn",
            "-ac",
            "1",
            "-ar",
            "16000",
            str(audio_path),
        ]
        subprocess.run(command, check=True)

    def _run_whisper(
        self,
        audio_path: Path,
        output_path: Path,
        model_path: Path,
        language: str,
    ) -> None:
        workdir = output_path.parent
        model_rel = Path(self._relative_posix_path(model_path, workdir))
        audio_rel = Path(self._relative_posix_path(audio_path, workdir))
        output_rel = output_path.name
        filter_arg = (
            f"whisper=model={model_rel.as_posix()}:"
            f"language={language}:"
            f"format=json:"
            f"destination={output_rel}:"
            "use_gpu=false"
        )
        command = [
            "ffmpeg",
            "-hide_banner",
            "-loglevel",
            "error",
            "-y",
            "-i",
            audio_rel.as_posix(),
            "-af",
            filter_arg,
            "-f",
            "null",
            "-",
        ]
        subprocess.run(command, cwd=workdir, check=True)

    def _run_funasr_sensevoice(
        self,
        audio_path: Path,
        output_path: Path,
        model_name: str,
    ) -> None:
        runtime_dir = self._ensure_funasr_runtime()
        model_path = self._ensure_funasr_model(model_name)
        chunks = self._detect_audio_chunks(audio_path)
        chunk_dir = output_path.parent / ".sensevoice_chunks"
        chunk_dir.mkdir(parents=True, exist_ok=True)

        lines: list[str] = []
        for index, (start_sec, end_sec) in enumerate(chunks):
            chunk_audio = chunk_dir / f"chunk-{index:03d}.wav"
            self._extract_audio_chunk(audio_path, chunk_audio, start_sec, end_sec)
            text = self._run_funasr_binary(
                binary_path=runtime_dir / "llama-funasr-sensevoice.exe",
                model_path=model_path,
                audio_path=chunk_audio,
            )
            cleaned = self._clean_text(text)
            if not cleaned:
                continue
            lines.append(
                json.dumps(
                    {
                        "start": int(round(start_sec * 1000)),
                        "end": int(round(end_sec * 1000)),
                        "text": cleaned,
                    },
                    ensure_ascii=False,
                )
            )

        if not lines:
            raise RuntimeError("SenseVoice transcription produced no transcript segments")
        output_path.write_text("\n".join(lines), encoding="utf-8")

    def _ensure_funasr_runtime(self) -> Path:
        runtime_dir = self.funasr_runtime_root / "windows-x64"
        if runtime_dir.exists():
            return runtime_dir

        self.funasr_runtime_root.mkdir(parents=True, exist_ok=True)
        archive_path = self.funasr_runtime_root / "funasr-llamacpp-windows-x64.zip"
        if not archive_path.exists():
            urllib.request.urlretrieve(FUNASR_RUNTIME_URL, archive_path)
        with zipfile.ZipFile(archive_path, "r") as archive:
            archive.extractall(runtime_dir)
        return runtime_dir

    def _ensure_funasr_model(self, model_name: str) -> Path:
        config = FUNASR_MODEL_CONFIGS.get(model_name)
        if config is None:
            supported = ", ".join(sorted(FUNASR_MODEL_CONFIGS))
            raise ValueError(f"Unsupported FunASR model '{model_name}'. Supported: {supported}")

        self.funasr_model_root.mkdir(parents=True, exist_ok=True)
        model_path = self.funasr_model_root / model_name
        if not model_path.exists():
            url = (
                f"https://huggingface.co/{config['repo_id']}/resolve/main/{config['file_name']}"
            )
            urllib.request.urlretrieve(url, model_path)
        return model_path

    def _detect_audio_chunks(self, audio_path: Path) -> list[tuple[float, float]]:
        duration_sec = self._probe_duration(audio_path)
        if duration_sec is None:
            raise RuntimeError(f"Unable to detect duration for audio: {audio_path}")

        command = [
            "ffmpeg",
            "-hide_banner",
            "-i",
            str(audio_path),
            "-af",
            "silencedetect=n=-35dB:d=0.35",
            "-f",
            "null",
            "-",
        ]
        result = subprocess.run(
            command,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            check=False,
        )
        stderr = result.stderr or ""

        silence_starts = [float(match) for match in re.findall(r"silence_start: ([0-9.]+)", stderr)]
        silence_ends = [float(match) for match in re.findall(r"silence_end: ([0-9.]+)", stderr)]
        intervals = list(zip(silence_starts, silence_ends, strict=False))

        speech_chunks: list[tuple[float, float]] = []
        cursor = 0.0
        for silence_start, silence_end in intervals:
            if silence_start > cursor:
                speech_chunks.append((cursor, silence_start))
            cursor = max(cursor, silence_end)
        if cursor < duration_sec:
            speech_chunks.append((cursor, duration_sec))

        if not speech_chunks:
            speech_chunks = [(0.0, duration_sec)]

        return self._normalize_chunks(speech_chunks)

    def _normalize_chunks(self, chunks: list[tuple[float, float]]) -> list[tuple[float, float]]:
        normalized: list[tuple[float, float]] = []
        max_duration = 18.0
        min_duration = 1.2

        for start_sec, end_sec in chunks:
            start_sec = max(0.0, start_sec)
            end_sec = max(start_sec, end_sec)
            duration = end_sec - start_sec
            if duration <= 0.15:
                continue

            if duration <= max_duration:
                if normalized and duration < min_duration:
                    prev_start, prev_end = normalized[-1]
                    normalized[-1] = (prev_start, end_sec)
                else:
                    normalized.append((start_sec, end_sec))
                continue

            cursor = start_sec
            while cursor < end_sec:
                piece_end = min(end_sec, cursor + max_duration)
                normalized.append((cursor, piece_end))
                cursor = piece_end

        if not normalized:
            return chunks
        return normalized

    def _extract_audio_chunk(
        self,
        audio_path: Path,
        output_path: Path,
        start_sec: float,
        end_sec: float,
    ) -> None:
        command = [
            "ffmpeg",
            "-hide_banner",
            "-loglevel",
            "error",
            "-y",
            "-ss",
            str(max(0.0, start_sec)),
            "-to",
            str(max(start_sec, end_sec)),
            "-i",
            str(audio_path),
            "-ac",
            "1",
            "-ar",
            "16000",
            str(output_path),
        ]
        subprocess.run(command, check=True)

    def _run_funasr_binary(
        self,
        binary_path: Path,
        model_path: Path,
        audio_path: Path,
    ) -> str:
        command = [
            str(binary_path),
            "-m",
            str(model_path),
            "-a",
            str(audio_path),
        ]
        completed = subprocess.run(
            command,
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
        )
        return completed.stdout.strip()

    def _relative_posix_path(self, target: Path, cwd: Path) -> str:
        return Path(os.path.relpath(target, cwd)).as_posix()

    def _parse_transcript_jsonl(self, transcript_jsonl: Path) -> list[TranscriptSegment]:
        segments: list[TranscriptSegment] = []
        for index, raw_line in enumerate(transcript_jsonl.read_text(encoding="utf-8").splitlines()):
            line = raw_line.strip()
            if not line:
                continue
            payload = json.loads(line)
            text = self._clean_text(payload.get("text", ""))
            if not text:
                continue
            segments.append(
                TranscriptSegment(
                    segment_index=index,
                    start_sec=float(payload["start"]) / 1000.0,
                    end_sec=float(payload["end"]) / 1000.0,
                    text=text,
                )
            )
        return segments

    def _clean_text(self, text: str) -> str:
        normalized = re.sub(r"\s+", " ", text).strip()
        normalized = normalized.replace(" ,", "，").replace(",", "，")
        return normalized

    def _render_transcript_text(self, segments: list[TranscriptSegment]) -> str:
        lines = [
            f"[{segment.timestamp}] {segment.text}"
            for segment in segments
        ]
        return "\n".join(lines)

    def _build_units(
        self,
        transcript_segments: list[TranscriptSegment],
        problem_slug: str,
        problem_display_name: str,
    ) -> list[KnowledgeUnitCandidate]:
        if not transcript_segments:
            return []

        grouped_segments: list[list[TranscriptSegment]] = []
        current_group: list[TranscriptSegment] = []
        current_types: list[str] = []

        for segment in transcript_segments:
            segment_type = self._classify_text(segment.text)
            if current_group and self._should_split_group(
                current_group,
                current_types,
                segment,
                segment_type,
            ):
                grouped_segments.append(current_group)
                current_group = []
                current_types = []

            current_group.append(segment)
            current_types.append(segment_type)

        if current_group:
            grouped_segments.append(current_group)

        units: list[KnowledgeUnitCandidate] = []
        type_counters: Counter[str] = Counter()
        for index, group in enumerate(grouped_segments, start=1):
            texts = [segment.text for segment in group]
            combined_text = " ".join(texts)
            dominant_type = self._dominant_type(texts)
            type_counters[dominant_type] += 1
            summary = self._make_summary(combined_text)
            title = self._make_title(
                problem_display_name=problem_display_name,
                unit_type=dominant_type,
                sequence=type_counters[dominant_type],
                combined_text=combined_text,
            )
            category = (
                f"exercise.{problem_slug}"
                if dominant_type == "exercise"
                else f"posture.{problem_slug}"
            )
            body_lines = [
                "## 自动转录摘录",
                *[
                    f"- [{segment.timestamp}] {segment.text}"
                    for segment in group
                ],
            ]
            if dominant_type == "warning":
                body_lines.append("\n## 备注\n- 该片段包含风险或错误动作提醒，回答时应保守引用。")

            units.append(
                KnowledgeUnitCandidate(
                    unit_key=f"{slugify(problem_slug)}-{dominant_type}-{index:02d}",
                    problem_slug=problem_slug,
                    problem_display_name=problem_display_name,
                    category=category,
                    unit_type=dominant_type,
                    title=title,
                    summary=summary,
                    body_markdown="\n".join(body_lines),
                    source_start_sec=group[0].start_sec,
                    source_end_sec=group[-1].end_sec,
                    evidence_segment_indices=[segment.segment_index for segment in group],
                    tags=self._make_tags(problem_slug, dominant_type, combined_text),
                    transcript_excerpt=combined_text[:240],
                )
            )

        return units

    def _should_split_group(
        self,
        current_group: list[TranscriptSegment],
        current_types: list[str],
        next_segment: TranscriptSegment,
        next_type: str,
    ) -> bool:
        current_duration = current_group[-1].end_sec - current_group[0].start_sec
        current_chars = sum(len(segment.text) for segment in current_group)
        dominant = Counter(current_types).most_common(1)[0][0]
        transition_match = re.match(r"^(首先|然后|接下来|最后|再来|那么|所以)", next_segment.text)

        if current_duration >= 42 or current_chars >= 180:
            return True
        if transition_match and current_duration >= 8:
            return True
        if (
            next_type in {"self_check", "exercise"}
            and next_type != dominant
            and current_duration >= 5
        ):
            return True
        if next_type != dominant and current_duration >= 8 and current_chars >= 35:
            return True
        return False

    def _classify_text(self, text: str) -> str:
        rules = [
            (
                "self_check",
                [
                    "自测",
                    "测试",
                    "筛查",
                    "摔查",
                    "判断",
                    "从侧面看",
                    "侧面看",
                    "自然放松",
                    "站姿",
                    "耳垂",
                    "肩峰",
                    "前方",
                    "观察",
                ],
            ),
            (
                "exercise",
                [
                    "动作",
                    "训练",
                    "练习",
                    "拉伸",
                    "放松",
                    "激活",
                    "回收",
                    "后缩",
                    "伸展",
                    "处理方法",
                    "第一步",
                    "第二步",
                    "第三步",
                    "准备一个",
                    "保留30秒",
                    "两组",
                    "网球",
                    "毛巾",
                    "旋转",
                    "强化",
                    "关节松动",
                ],
            ),
            ("warning", ["不要", "避免", "隐患", "疼", "酸痛", "不适", "错误", "代偿", "严重"]),
            ("cause", ["因为", "导致", "造成", "影响", "问题", "长期", "习惯"]),
        ]
        for unit_type, keywords in rules:
            if any(keyword in text for keyword in keywords):
                return unit_type
        return "explanation"

    def _dominant_type(self, texts: list[str]) -> str:
        votes = Counter(self._classify_text(text) for text in texts)
        return votes.most_common(1)[0][0]

    def _make_summary(self, combined_text: str) -> str:
        compact = re.sub(r"[。！？!?.]+", "。", combined_text)
        summary = compact[:90].strip()
        return summary if len(compact) <= 90 else f"{summary}..."

    def _make_title(
        self,
        problem_display_name: str,
        unit_type: str,
        sequence: int,
        combined_text: str,
    ) -> str:
        templates = {
            "explanation": "原理说明",
            "self_check": "自测与判断",
            "exercise": "改善动作",
            "cause": "影响与成因",
            "warning": "风险提醒",
        }
        focus = re.sub(r"[，。！？!?\s]+", "", combined_text)[:14]
        suffix = templates.get(unit_type, "内容整理")
        return f"{problem_display_name}{suffix} {sequence}: {focus}"

    def _make_tags(self, problem_slug: str, unit_type: str, combined_text: str) -> list[str]:
        tags = [problem_slug, unit_type]
        if "头前" in combined_text or "头前移" in combined_text:
            tags.append("头前移")
        if "训练" in combined_text or "动作" in combined_text:
            tags.append("动作演示")
        if "自然放松" in combined_text or "判断" in combined_text:
            tags.append("自测")
        return sorted(set(tags))

    def _export_clips(
        self,
        video_path: Path,
        artifact_dir: Path,
        units: list[KnowledgeUnitCandidate],
        export_clips: bool,
    ) -> list[KnowledgeClipCandidate]:
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
                    str(clip_path),
                ]
                subprocess.run(command, check=True)

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
