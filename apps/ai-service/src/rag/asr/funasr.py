"""FunASR SenseVoice ASR provider — local inference via llama.cpp runtime."""

from __future__ import annotations

import json
import re
import subprocess
import urllib.request
import zipfile
from pathlib import Path

from ..knowledge_pack import TranscriptSegment
from .base import ASRProvider

# FunASR is an open-source ASR framework from Alibaba DAMO Academy.
# SenseVoice is its Chinese ASR model. Here we use a llama.cpp-based Windows build.
_FUNASR_RUNTIME_VERSION = "runtime-llamacpp-v0.1.1"
_FUNASR_RUNTIME_URL = (
    "https://github.com/modelscope/FunASR/releases/download/"
    f"{_FUNASR_RUNTIME_VERSION}/funasr-llamacpp-windows-x64.zip"
)

# FunASR SenseVoice model configuration.
# gguf is a lightweight quantized format; q8 = 8-bit quantization (small, fast).
FUNASR_MODEL_CONFIGS: dict[str, dict[str, str]] = {
    "sensevoice-small-q8.gguf": {
        "binary_name": "llama-funasr-sensevoice.exe",
        "repo_id": "FunAudioLLM/SenseVoiceSmall-GGUF",
        "file_name": "sensevoice-small-q8.gguf",
    },
}

_DEFAULT_MODEL = "sensevoice-small-q8.gguf"


class FunASRProvider(ASRProvider):
    """ASR using FunASR SenseVoice via a local llama.cpp-based runtime.

    Unlike whisper.cpp, SenseVoice's C++ runtime cannot process long audio in one pass.
    The pipeline:
    1. Detect silence boundaries with ffmpeg's silencedetect filter
    2. Split audio into chunks (max 18s each)
    3. Run SenseVoice inference on each chunk
    4. Concatenate results into unified JSONL output
    """

    name = "funasr_sensevoice"

    def __init__(
        self,
        data_root: Path | None = None,
        model_name: str | None = None,
    ):
        self.data_root = Path(data_root or _default_data_root())
        self.model_name = model_name or _DEFAULT_MODEL
        self._runtime_root = self.data_root / ".cache" / "funasr_runtime"
        self._model_root = self.data_root / ".cache" / "funasr_models"

    async def transcribe(
        self,
        audio_path: Path,
        language: str = "zh",
    ) -> list[TranscriptSegment]:
        audio_path = Path(audio_path).resolve()
        if not audio_path.exists():
            raise FileNotFoundError(f"Audio file not found: {audio_path}")

        runtime_dir = self._ensure_runtime()
        model_path = self._ensure_model(self.model_name)
        chunks = _detect_audio_chunks(audio_path)

        chunk_dir = audio_path.parent / ".sensevoice_chunks"
        chunk_dir.mkdir(parents=True, exist_ok=True)

        segments: list[TranscriptSegment] = []
        binary_path = runtime_dir / "llama-funasr-sensevoice.exe"

        for index, (start_sec, end_sec) in enumerate(chunks):
            chunk_audio = chunk_dir / f"chunk-{index:03d}.wav"
            _extract_audio_chunk(audio_path, chunk_audio, start_sec, end_sec)

            text = _run_funasr_binary(binary_path, model_path, chunk_audio)
            cleaned = _clean_text(text)
            if not cleaned:
                continue

            segments.append(
                TranscriptSegment(
                    segment_index=index,
                    start_sec=start_sec,
                    end_sec=end_sec,
                    text=cleaned,
                )
            )

        if not segments:
            raise RuntimeError("SenseVoice transcription produced no transcript segments")

        # Write JSONL output for compatibility with the rest of the pipeline
        output_path = audio_path.parent / "transcript.raw.jsonl"
        lines = [
            json.dumps(
                {
                    "start": int(round(s.start_sec * 1000)),
                    "end": int(round(s.end_sec * 1000)),
                    "text": s.text,
                },
                ensure_ascii=False,
            )
            for s in segments
        ]
        output_path.write_text("\n".join(lines), encoding="utf-8")

        return segments

    def _ensure_runtime(self) -> Path:
        """Ensure FunASR C++ runtime exists, downloading and extracting if needed."""
        runtime_dir = self._runtime_root / "windows-x64"
        if runtime_dir.exists():
            return runtime_dir

        self._runtime_root.mkdir(parents=True, exist_ok=True)
        archive_path = self._runtime_root / "funasr-llamacpp-windows-x64.zip"

        if not archive_path.exists():
            urllib.request.urlretrieve(_FUNASR_RUNTIME_URL, archive_path)

        with zipfile.ZipFile(archive_path, "r") as archive:
            archive.extractall(runtime_dir)
        return runtime_dir

    def _ensure_model(self, model_name: str) -> Path:
        """Ensure FunASR SenseVoice model file exists, downloading if needed."""
        config = FUNASR_MODEL_CONFIGS.get(model_name)
        if config is None:
            supported = ", ".join(sorted(FUNASR_MODEL_CONFIGS))
            raise ValueError(f"Unsupported FunASR model '{model_name}'. Supported: {supported}")

        self._model_root.mkdir(parents=True, exist_ok=True)
        model_path = self._model_root / model_name

        if not model_path.exists():
            url = f"https://huggingface.co/{config['repo_id']}/resolve/main/{config['file_name']}"
            urllib.request.urlretrieve(url, model_path)
        return model_path


# ---------------------------------------------------------------------------
# Shared utilities
# ---------------------------------------------------------------------------


def _default_data_root() -> Path:
    """Default data root: apps/ai-service/data/ (two levels up from this file)."""
    return Path(__file__).resolve().parents[2] / "data"


def _detect_audio_chunks(audio_path: Path) -> list[tuple[float, float]]:
    """Split long audio into speech segments using ffmpeg's silence detection.

    Silence detection parameters:
        n=-35dB  — Volume below -35dB is considered silence (suitable for human speech)
        d=0.35   — Silence must last at least 0.35 seconds to be a real pause
    """
    duration_sec = _probe_duration(audio_path)
    if duration_sec is None:
        raise RuntimeError(f"Unable to detect duration for audio: {audio_path}")

    command = [
        "ffmpeg",
        "-hide_banner",
        "-i", str(audio_path),
        "-af", "silencedetect=n=-35dB:d=0.35",
        "-f", "null",
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

    silence_starts = [float(m) for m in re.findall(r"silence_start: ([0-9.]+)", stderr)]
    silence_ends = [float(m) for m in re.findall(r"silence_end: ([0-9.]+)", stderr)]
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

    return _normalize_chunks(speech_chunks)


def _normalize_chunks(chunks: list[tuple[float, float]]) -> list[tuple[float, float]]:
    """Normalize audio segments: filter too-short, split too-long.

    Rules:
        - Segments <= 0.15s are discarded (likely noise)
        - Segments <= 18s are kept as-is
        - Segments > 18s are split into 18s sub-segments
    """
    max_duration = 18.0
    normalized: list[tuple[float, float]] = []

    for start_sec, end_sec in chunks:
        start_sec = max(0.0, start_sec)
        end_sec = max(start_sec, end_sec)
        duration = end_sec - start_sec

        if duration <= 0.15:
            continue

        if duration <= max_duration:
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
    audio_path: Path,
    output_path: Path,
    start_sec: float,
    end_sec: float,
) -> None:
    """Extract a time range from an audio file using ffmpeg."""
    command = [
        "ffmpeg",
        "-hide_banner",
        "-loglevel", "error",
        "-y",
        "-ss", str(max(0.0, start_sec)),
        "-to", str(max(start_sec, end_sec)),
        "-i", str(audio_path),
        "-ac", "1",
        "-ar", "16000",
        str(output_path),
    ]
    subprocess.run(command, check=True)


def _run_funasr_binary(
    binary_path: Path,
    model_path: Path,
    audio_path: Path,
) -> str:
    """Run FunASR SenseVoice binary on a single audio chunk.

    Command format: llama-funasr-sensevoice.exe -m <model> -a <audio>
    Output: recognized text (stdout).
    """
    command = [
        str(binary_path),
        "-m", str(model_path),
        "-a", str(audio_path),
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


def _probe_duration(media_path: Path) -> float | None:
    """Probe media duration using ffprobe."""
    command = [
        "ffprobe",
        "-v", "error",
        "-show_entries", "format=duration",
        "-of", "default=noprint_wrappers=1:nokey=1",
        str(media_path),
    ]
    result = subprocess.run(command, check=True, capture_output=True, text=True)
    value = result.stdout.strip()
    return float(value) if value else None


def _clean_text(text: str) -> str:
    """Clean ASR output text."""
    normalized = re.sub(r"\s+", " ", text).strip()
    normalized = normalized.replace(" ,", "，").replace(",", "，")
    return normalized
