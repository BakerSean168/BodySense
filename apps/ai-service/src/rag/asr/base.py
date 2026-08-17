"""ASR provider abstraction for the video-to-knowledge pipeline."""

from __future__ import annotations

from pathlib import Path
from typing import Protocol

from ..knowledge_pack import TranscriptSegment


class ASRProvider(Protocol):
    """Protocol for ASR (Automatic Speech Recognition) providers.

    Each provider takes an audio file and returns a list of TranscriptSegment.
    Implementations can be local (whisper.cpp, FunASR) or remote (Whisper API).
    """

    name: str
    model_name: str | None = None

    async def transcribe(
        self,
        audio_path: Path,
        language: str = "zh",
    ) -> list[TranscriptSegment]:
        """Transcribe an audio file into structured segments.

        Args:
            audio_path: Path to a 16kHz mono WAV file.
            language: Language code (e.g. "zh", "en").

        Returns:
            Ordered list of TranscriptSegment with start/end times in seconds.
        """
        ...
