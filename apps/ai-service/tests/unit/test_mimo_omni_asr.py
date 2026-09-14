from __future__ import annotations

from pathlib import Path

import pytest

from src.rag.asr.base import ASRTranscriptionError
from src.rag.asr.mimo_omni import MiMoOmniASRProvider
from src.rag.knowledge_pack import TranscriptSegment


@pytest.mark.asyncio
async def test_chunk_failure_never_returns_a_partial_transcript(
    tmp_path: Path, monkeypatch
) -> None:
    audio = tmp_path / "source.wav"
    audio.write_bytes(b"source")
    chunk_one = tmp_path / "chunk-1.wav"
    chunk_two = tmp_path / "chunk-2.wav"
    chunk_one.write_bytes(b"one")
    chunk_two.write_bytes(b"two")

    monkeypatch.setattr(
        "src.rag.asr.mimo_omni._ffmpeg_split",
        lambda _path, _duration: [chunk_one, chunk_two],
    )
    provider = MiMoOmniASRProvider(api_key="test-key")
    calls = 0

    async def fake_transcribe_chunk(
        _chunk_bytes: bytes,
        _language: str,
        offset_sec: float = 0.0,
    ) -> list[TranscriptSegment]:
        nonlocal calls
        calls += 1
        if calls == 2:
            raise RuntimeError("provider transport failed")
        return [
            TranscriptSegment(
                segment_index=0,
                start_sec=offset_sec,
                end_sec=offset_sec + 1,
                text="first chunk",
            )
        ]

    monkeypatch.setattr(provider, "_transcribe_chunk", fake_transcribe_chunk)

    with pytest.raises(ASRTranscriptionError, match="chunk 2/2 failed"):
        await provider.transcribe(audio)

    assert not chunk_one.exists()
    assert not chunk_two.exists()
    assert not (tmp_path / "transcript.raw.jsonl").exists()
