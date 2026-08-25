from __future__ import annotations

import hashlib

import pytest
from fastapi import HTTPException

from src.api.routes import knowledge as knowledge_routes


@pytest.mark.asyncio
async def test_video_ingestion_rejects_content_that_differs_from_registered_hash(
    tmp_path, monkeypatch
) -> None:
    source_dir = tmp_path / "sources"
    source_dir.mkdir()
    source_file = source_dir / "video.mp4"
    source_file.write_bytes(b"registered bytes changed")
    monkeypatch.setattr(knowledge_routes, "_DATA_ROOT", tmp_path)

    request = knowledge_routes.IngestVideoRequestModel(
        source_key="video-forward-head-v1",
        expected_content_hash=hashlib.sha256(b"original registered bytes").hexdigest(),
        video_path="sources/video.mp4",
        problem_slug="forward-head",
        problem_display_name="Forward Head",
        author="operator",
        source_title=None,
        language="zh",
        transcript_provider="whisper.cpp",
        transcript_model=None,
        whisper_model="ggml-base.bin",
    )

    with pytest.raises(HTTPException) as exc:
        await knowledge_routes.ingest_video(request)
    assert exc.value.status_code == 409
    assert "content hash" in str(exc.value.detail)
