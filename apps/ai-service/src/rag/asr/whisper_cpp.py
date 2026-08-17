"""Whisper.cpp ASR provider — local inference via ffmpeg whisper filter."""

from __future__ import annotations

import json
import logging
import os
import subprocess
import urllib.request
from pathlib import Path

from ..knowledge_pack import TranscriptSegment
from .base import ASRProvider

logger = logging.getLogger(__name__)

# whisper.cpp is the C++ implementation of OpenAI Whisper, suitable for offline inference.
# Larger models are more accurate but slower. tiny is fastest, small is most accurate.
WHISPER_MODEL_URLS: dict[str, str] = {
    "ggml-tiny.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
    "ggml-base.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
    "ggml-small.bin": "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
}

DEFAULT_MODEL = os.getenv("ASR_WHISPER_MODEL", "ggml-base.bin")


def _default_data_root() -> Path:
    """Default data root: apps/ai-service/data/ (two levels up from this file)."""
    return Path(__file__).resolve().parents[2] / "data"


class WhisperCppProvider(ASRProvider):
    """ASR using whisper.cpp via ffmpeg's built-in whisper filter.

    ffmpeg has a built-in whisper filter that can call whisper.cpp models directly,
    without needing to compile whisper.cpp as a standalone binary. It:
    1. Reads audio
    2. Splits into 30-second windows
    3. Feeds each window into the whisper model for inference
    4. Outputs JSON transcription results (with start/end/text)
    """

    name = "whisper.cpp"

    def __init__(
        self,
        data_root: Path | None = None,
        model_name: str | None = None,
    ):
        self.data_root = Path(data_root or _default_data_root())
        self.model_name = model_name or DEFAULT_MODEL
        self._whisper_root = self.data_root / ".cache" / "whisper"

    async def transcribe(
        self,
        audio_path: Path,
        language: str = "zh",
    ) -> list[TranscriptSegment]:
        audio_path = Path(audio_path).resolve()
        if not audio_path.exists():
            raise FileNotFoundError(f"Audio file not found: {audio_path}")

        model_path = self._ensure_model(self.model_name or DEFAULT_MODEL)
        workdir = audio_path.parent
        output_path = workdir / "transcript.raw.jsonl"

        self._run_whisper(
            audio_path=audio_path,
            output_path=output_path,
            model_path=model_path,
            language=language,
            workdir=workdir,
        )

        return _parse_transcript_jsonl(output_path)

    def _ensure_model(self, model_name: str) -> Path:
        """Ensure whisper.cpp model file exists, downloading from Hugging Face if needed."""
        if model_name not in WHISPER_MODEL_URLS:
            supported = ", ".join(sorted(WHISPER_MODEL_URLS))
            raise ValueError(f"Unsupported whisper model '{model_name}'. Supported: {supported}")

        self._whisper_root.mkdir(parents=True, exist_ok=True)
        model_path = self._whisper_root / model_name

        if not model_path.exists():
            url = WHISPER_MODEL_URLS[model_name]
            logger.info("Downloading whisper model %s from %s", model_name, url)
            urllib.request.urlretrieve(url, model_path)
            logger.info("Downloaded whisper model to %s", model_path)
        return model_path

    def _run_whisper(
        self,
        audio_path: Path,
        output_path: Path,
        model_path: Path,
        language: str,
        workdir: Path,
    ) -> None:
        """Run whisper.cpp via ffmpeg's whisper filter."""
        model_rel = _relative_posix_path(model_path, workdir)
        audio_rel = _relative_posix_path(audio_path, workdir)

        filter_arg = (
            f"whisper=model={model_rel}:"
            f"language={language}:"
            f"format=json:"
            f"destination={output_path.name}:"
            "use_gpu=false"
        )
        command = [
            "ffmpeg",
            "-hide_banner",
            "-loglevel",
            "error",
            "-y",
            "-i",
            audio_rel,
            "-af",
            filter_arg,
            "-f",
            "null",
            "-",
        ]
        subprocess.run(command, cwd=workdir, check=True)


# ---------------------------------------------------------------------------
# Shared utilities
# ---------------------------------------------------------------------------


def _relative_posix_path(target: Path, cwd: Path) -> str:
    """Compute target relative to cwd, converted to POSIX format (forward slashes)."""
    return Path(os.path.relpath(target, cwd)).as_posix()


def _parse_transcript_jsonl(transcript_jsonl: Path) -> list[TranscriptSegment]:
    """Parse an ASR JSONL file into structured TranscriptSegment list.

    JSONL format (one JSON object per line):
        {"start": 0, "end": 5200, "text": "..."}
        {"start": 5200, "end": 12800, "text": "..."}

    Note: start/end are in milliseconds and will be converted to seconds.
    """
    import re

    def _clean_text(text: str) -> str:
        normalized = re.sub(r"\s+", " ", text).strip()
        normalized = normalized.replace(" ,", "，").replace(",", "，")
        return normalized

    segments: list[TranscriptSegment] = []
    for index, raw_line in enumerate(transcript_jsonl.read_text(encoding="utf-8").splitlines()):
        line = raw_line.strip()
        if not line:
            continue
        payload = json.loads(line)
        text = _clean_text(payload.get("text", ""))
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
