"""ASR API provider — transcription via OpenAI-compatible remote API."""

from __future__ import annotations

import os
from pathlib import Path

from ..knowledge_pack import TranscriptSegment
from .base import ASRProvider

# Environment variables for API configuration
_ENV_API_KEY = "ASR_API_KEY"
_ENV_BASE_URL = "ASR_API_BASE_URL"
_ENV_MODEL = "ASR_API_MODEL"


class ASRAPIProvider(ASRProvider):
    """ASR using an OpenAI-compatible audio transcription API.

    Works with any service exposing the /v1/audio/transcriptions endpoint:
        - MiMo ASR (mimo-v2.5-asr)
        - OpenAI Whisper API
        - LocalAI (self-hosted)
        - faster-whisper-server

    The provider requests ``verbose_json`` response format to obtain per-segment
    timestamps, which are mapped to ``TranscriptSegment`` objects.
    """

    name = "asr_api"

    def __init__(
        self,
        api_key: str | None = None,
        base_url: str | None = None,
        model: str | None = None,
    ):
        self.api_key = api_key or os.getenv(_ENV_API_KEY) or os.getenv("OPENAI_API_KEY")
        self.base_url = base_url or os.getenv(_ENV_BASE_URL)
        self.model = model or os.getenv(_ENV_MODEL, "mimo-v2.5-asr")

        if not self.api_key:
            raise ValueError(f"{_ENV_API_KEY} or OPENAI_API_KEY is required for asr_api provider")

        self._client = None

    def _get_client(self):
        """Lazy-initialize the OpenAI async client."""
        if self._client is None:
            from openai import AsyncOpenAI

            assert self.api_key is not None
            if self.base_url:
                self._client = AsyncOpenAI(api_key=self.api_key, base_url=self.base_url)
            else:
                self._client = AsyncOpenAI(api_key=self.api_key)
        return self._client

    async def transcribe(
        self,
        audio_path: Path,
        language: str = "zh",
    ) -> list[TranscriptSegment]:
        audio_path = Path(audio_path).resolve()
        if not audio_path.exists():
            raise FileNotFoundError(f"Audio file not found: {audio_path}")

        client = self._get_client()

        with open(audio_path, "rb") as audio_file:
            response = await client.audio.transcriptions.create(
                model=self.model,
                file=audio_file,
                language=language,
                response_format="verbose_json",
                timestamp_granularities=["segment"],
            )

        segments: list[TranscriptSegment] = []

        if hasattr(response, "segments") and response.segments:
            for index, seg in enumerate(response.segments):
                text = _clean_text(seg.text)
                if not text:
                    continue
                segments.append(
                    TranscriptSegment(
                        segment_index=index,
                        start_sec=float(seg.start),
                        end_sec=float(seg.end),
                        text=text,
                    )
                )
        else:
            # Fallback: treat the whole text as one segment
            text = _clean_text(response.text)
            if text:
                segments.append(
                    TranscriptSegment(
                        segment_index=0,
                        start_sec=0.0,
                        end_sec=0.0,
                        text=text,
                    )
                )

        if not segments:
            raise RuntimeError("ASR API returned no transcript segments")

        # Write JSONL for compatibility with the rest of the pipeline
        import json

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


def _clean_text(text: str) -> str:
    """Clean ASR output text."""
    import re

    normalized = re.sub(r"\s+", " ", text).strip()
    normalized = normalized.replace(" ,", "，").replace(",", "，")
    return normalized
