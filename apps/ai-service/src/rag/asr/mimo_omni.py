"""MiMo V2.5 Omni ASR provider — transcription via chat completions with audio input.

Uses the MiMo-V2.5 omni multimodal model through the standard chat completions API.
Audio is sent as base64-encoded input_audio in the message content.
Long audio is split into chunks using ffmpeg for processing.
"""

from __future__ import annotations

import base64
import json
import logging
import os
import subprocess
import tempfile
from pathlib import Path

import httpx

from ..knowledge_pack import TranscriptSegment
from .base import ASRProvider

logger = logging.getLogger(__name__)

# Environment variables
_ENV_API_KEY = "ASR_API_KEY"
_ENV_BASE_URL = "ASR_API_BASE_URL"
_ENV_MODEL = "ASR_API_MODEL"

# Chunk duration in seconds
_CHUNK_DURATION_SEC = 30

# ffmpeg output format: mono 16kHz 16-bit WAV
_SAMPLE_RATE = 16000
_BYTES_PER_SAMPLE = 2  # 16-bit


class MiMoOmniASRProvider(ASRProvider):
    """ASR using MiMo-V2.5 omni model via chat completions API."""

    name = "mimo_omni"

    def __init__(
        self,
        api_key: str | None = None,
        base_url: str | None = None,
        model: str | None = None,
    ):
        self.api_key = (
            api_key
            or os.getenv(_ENV_API_KEY)
            or os.getenv("OPENAI_API_KEY")
        )
        self.base_url = base_url or os.getenv(_ENV_BASE_URL, "https://token-plan-cn.xiaomimimo.com/v1")
        self.model = model or os.getenv(_ENV_MODEL, "mimo-v2.5")

        if not self.api_key:
            raise ValueError(f"{_ENV_API_KEY} is required for mimo_omni provider")

    async def transcribe(
        self,
        audio_path: Path,
        language: str = "zh",
    ) -> list[TranscriptSegment]:
        audio_path = Path(audio_path).resolve()
        if not audio_path.exists():
            raise FileNotFoundError(f"Audio file not found: {audio_path}")

        # Split audio into chunks using ffmpeg
        chunk_paths = _ffmpeg_split(audio_path, _CHUNK_DURATION_SEC)
        logger.info(f"Split audio into {len(chunk_paths)} chunks")

        # Process chunks sequentially (API has rate limits)
        all_segments: list[TranscriptSegment] = []
        segment_index = 0

        for i, chunk_path in enumerate(chunk_paths):
            offset_sec = i * _CHUNK_DURATION_SEC
            try:
                chunk_bytes = chunk_path.read_bytes()
                segments = await self._transcribe_chunk(
                    chunk_bytes,
                    language,
                    offset_sec=offset_sec,
                )
                for seg in segments:
                    all_segments.append(
                        TranscriptSegment(
                            segment_index=segment_index,
                            start_sec=seg.start_sec,
                            end_sec=seg.end_sec,
                            text=seg.text,
                        )
                    )
                    segment_index += 1
                logger.info(f"Chunk {i+1}/{len(chunk_paths)} done: {len(segments)} segments")
            except Exception as e:
                logger.error(f"Chunk {i+1}/{len(chunk_paths)} failed: {e}")
            finally:
                chunk_path.unlink(missing_ok=True)

        if not all_segments:
            raise RuntimeError("MiMo Omni ASR returned no transcript segments")

        # Write JSONL for pipeline compatibility
        _write_jsonl(all_segments, audio_path.parent / "transcript.raw.jsonl")

        return all_segments

    async def _transcribe_chunk(
        self,
        chunk_bytes: bytes,
        language: str,
        offset_sec: float = 0.0,
    ) -> list[TranscriptSegment]:
        """Transcribe a single audio chunk via chat completions."""
        audio_b64 = base64.b64encode(chunk_bytes).decode()

        # MiMo gateway auto-injects ASR prompt when it detects audio-only input.
        # DO NOT include text parts — the gateway rejects them with:
        # "ASR request must not include text parts; text prompt is injected by the gateway"
        payload = {
            "model": self.model,
            "messages": [
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "input_audio",
                            "input_audio": {
                                "data": audio_b64,
                                "format": "wav",
                            },
                        },
                    ],
                }
            ],
            "max_tokens": 4000,
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(timeout=120) as client:
            resp = await client.post(
                f"{self.base_url}/chat/completions",
                headers=headers,
                json=payload,
            )
            if resp.status_code != 200:
                logger.error(f"MiMo API error {resp.status_code}: {resp.text[:500]}")
                resp.raise_for_status()
            data = resp.json()

        text = data["choices"][0]["message"]["content"].strip()
        if not text:
            return []

        # Estimate duration from chunk size
        chunk_duration = len(chunk_bytes) / (_SAMPLE_RATE * _BYTES_PER_SAMPLE)

        # Return as a single segment per chunk (no fine-grained timestamps from omni model)
        return [
            TranscriptSegment(
                segment_index=0,
                start_sec=offset_sec,
                end_sec=offset_sec + chunk_duration,
                text=text,
            )
        ]


def _ffmpeg_split(audio_path: Path, chunk_sec: int) -> list[Path]:
    """Split audio into chunks using ffmpeg. Returns list of chunk file paths."""
    output_dir = Path(tempfile.mkdtemp(prefix="mimo_asr_"))

    # Get duration
    result = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration",
         "-of", "default=noprint_wrappers=1:nokey=1", str(audio_path)],
        capture_output=True, text=True, timeout=30,
    )
    duration = float(result.stdout.strip())

    if duration <= chunk_sec:
        # No need to split
        chunk_path = output_dir / "chunk_000.wav"
        subprocess.run(
            ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
             "-i", str(audio_path), "-ac", "1", "-ar", "16000", str(chunk_path)],
            timeout=120,
        )
        return [chunk_path]

    # Split into segments
    chunk_paths = []
    index = 0
    start = 0.0
    while start < duration:
        chunk_path = output_dir / f"chunk_{index:03d}.wav"
        subprocess.run(
            ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
             "-ss", str(start), "-t", str(chunk_sec),
             "-i", str(audio_path), "-ac", "1", "-ar", "16000", str(chunk_path)],
            timeout=120,
        )
        if chunk_path.exists() and chunk_path.stat().st_size > 0:
            chunk_paths.append(chunk_path)
        start += chunk_sec
        index += 1

    return chunk_paths


def _write_jsonl(segments: list[TranscriptSegment], output_path: Path) -> None:
    """Write segments to JSONL format for pipeline compatibility."""
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
