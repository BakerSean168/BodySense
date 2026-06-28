"""Knowledge library API routes."""

import os
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ...rag import (
    VideoIngestionPipeline,
    VideoIngestionRequest,
    get_knowledge_library,
)

# Allowed data root for video ingestion (path traversal defense)
_DEFAULT_DATA_ROOT = Path(__file__).resolve().parents[3] / "data"
_DATA_ROOT = Path(os.getenv("KNOWLEDGE_DATA_ROOT", str(_DEFAULT_DATA_ROOT))).resolve()

router = APIRouter(prefix="/api/knowledge", tags=["knowledge"])


class IngestVideoRequestModel(BaseModel):
    """Request to ingest a local video into the knowledge library."""

    video_path: str = Field(
        ...,
        min_length=1,
        description="Absolute local path to the source video",
    )
    problem_slug: str = Field(..., min_length=1, max_length=100)
    problem_display_name: str = Field(..., min_length=1, max_length=255)
    author: str = Field(..., min_length=1, max_length=255)
    source_title: Optional[str] = Field(None, max_length=500)
    language: str = Field("zh", min_length=2, max_length=20)
    transcript_provider: str = Field("whisper.cpp", min_length=1, max_length=100)
    transcript_model: Optional[str] = Field(None, max_length=200)
    whisper_model: str = Field("ggml-base.bin", min_length=1, max_length=100)
    force_transcribe: bool = False
    export_clips: bool = True
    overwrite_source: bool = False


class IngestVideoResponse(BaseModel):
    """Response after ingesting a video source."""

    source_id: Optional[int] = None
    source_key: str
    status: str
    artifact_dir: str
    transcript_segments: int
    knowledge_units: int
    clips: int


class SearchRequest(BaseModel):
    """Request to search the normalized knowledge library."""

    query: str = Field(..., min_length=1)
    top_k: int = Field(5, ge=1, le=20)
    problem_slug: Optional[str] = None
    unit_type: Optional[str] = None


class ClipResultItem(BaseModel):
    id: int
    clip_key: str
    clip_type: str
    title: str
    file_path: str
    source_timestamp: str


class SearchResultItem(BaseModel):
    id: int
    problem_slug: str
    category: str
    unit_type: str
    title: str
    summary: str
    body_markdown: str
    similarity: float
    source_title: str
    source_author: str
    source_timestamp: str
    tags: list[str]
    clips: list[ClipResultItem]


class SearchResponse(BaseModel):
    results: list[SearchResultItem]
    total: int


class StatsResponse(BaseModel):
    knowledge_sources: int
    knowledge_segments: int
    knowledge_units: int
    knowledge_clips: int


@router.post("/ingestions/video", response_model=IngestVideoResponse)
async def ingest_video(request: IngestVideoRequestModel):
    """Ingest one local video into the normalized knowledge library."""
    # Path traversal defense: reject absolute paths, '..' components, and paths outside data root
    video_path = Path(request.video_path)
    if video_path.is_absolute() or ".." in video_path.parts:
        raise HTTPException(
            status_code=400,
            detail="video_path must be a relative path without '..' components",
        )
    resolved = (_DATA_ROOT / video_path).resolve()
    if not str(resolved).startswith(str(_DATA_ROOT)):
        raise HTTPException(
            status_code=400,
            detail="video_path is outside the allowed data directory",
        )
    try:
        pipeline = VideoIngestionPipeline()
        pack = await pipeline.ingest(
            VideoIngestionRequest(
                video_path=request.video_path,
                problem_slug=request.problem_slug,
                problem_display_name=request.problem_display_name,
                author=request.author,
                source_title=request.source_title or Path(request.video_path).stem,
                language=request.language,
                transcript_provider=request.transcript_provider,
                transcript_model=request.transcript_model,
                whisper_model=request.whisper_model,
                force_transcribe=request.force_transcribe,
                export_clips=request.export_clips,
            )
        )

        library = get_knowledge_library()
        result = await library.ingest_generated_pack(
            pack,
            overwrite_source=request.overwrite_source,
        )
        return IngestVideoResponse(
            source_id=result.get("source_id"),
            source_key=result["source_key"],
            status=result["status"],
            artifact_dir=pack.artifact_dir,
            transcript_segments=len(pack.transcript_segments),
            knowledge_units=len(pack.units),
            clips=len(pack.clips),
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/search", response_model=SearchResponse)
async def search_knowledge(request: SearchRequest):
    """Search normalized knowledge units with semantic similarity."""
    try:
        library = get_knowledge_library()
        results = await library.search(
            query=request.query,
            top_k=request.top_k,
            problem_slug=request.problem_slug,
            unit_type=request.unit_type,
        )

        return SearchResponse(
            results=[
                SearchResultItem(
                    id=result.id,
                    problem_slug=result.problem_slug,
                    category=result.category,
                    unit_type=result.unit_type,
                    title=result.title,
                    summary=result.summary,
                    body_markdown=result.body_markdown,
                    similarity=result.similarity,
                    source_title=result.source_title,
                    source_author=result.source_author,
                    source_timestamp=result.source_timestamp,
                    tags=result.tags,
                    clips=[
                        ClipResultItem(
                            id=clip.id,
                            clip_key=clip.clip_key,
                            clip_type=clip.clip_type,
                            title=clip.title,
                            file_path=clip.file_path,
                            source_timestamp=clip.source_timestamp,
                        )
                        for clip in result.clips
                    ],
                )
                for result in results
            ],
            total=len(results),
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/sources")
async def list_sources():
    """List all ingested knowledge sources."""
    try:
        library = get_knowledge_library()
        return await library.list_sources()
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/stats", response_model=StatsResponse)
async def get_stats():
    """Get knowledge library statistics."""
    try:
        library = get_knowledge_library()
        stats = await library.stats()
        return StatsResponse(**stats)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
