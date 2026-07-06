"""ASR (Automatic Speech Recognition) provider subsystem."""

from __future__ import annotations

import os
from pathlib import Path

from .base import ASRProvider


def get_asr_provider(
    provider: str | None = None,
    data_root: Path | None = None,
) -> ASRProvider:
    """Create an ASR provider based on configuration.

    Priority: explicit ``provider`` argument > ``ASR_PROVIDER`` env var > ``whisper.cpp``.

    Args:
        provider: Provider name. One of ``whisper.cpp``, ``funasr_sensevoice``,
            ``asr_api``, ``mimo_omni``.
        data_root: Root directory for cached models and artifacts.

    Returns:
        An ASRProvider instance ready for transcription.
    """
    name = (provider or os.getenv("ASR_PROVIDER", "whisper.cpp")).strip().lower()

    if name == "whisper.cpp":
        from .whisper_cpp import WhisperCppProvider

        return WhisperCppProvider(data_root=data_root)

    if name == "funasr_sensevoice":
        from .funasr import FunASRProvider

        return FunASRProvider(data_root=data_root)

    if name == "asr_api":
        from .whisper_api import ASRAPIProvider

        return ASRAPIProvider()

    if name == "mimo_omni":
        from .mimo_omni import MiMoOmniASRProvider

        return MiMoOmniASRProvider()

    supported = "whisper.cpp, funasr_sensevoice, asr_api, mimo_omni"
    raise ValueError(f"Unsupported ASR provider '{name}'. Supported: {supported}")


__all__ = ["ASRProvider", "get_asr_provider"]
