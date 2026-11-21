"""Shared value objects and pool plumbing for the knowledge library modules."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from typing import Any

from psycopg import AsyncConnection
from psycopg.rows import TupleRow
from psycopg_pool import AsyncConnectionPool

from .embedding import EmbeddingGenerator
from .knowledge_pack import format_timestamp_range


@dataclass(frozen=True)
class ClipResult:
    """A clip associated with a search result."""

    id: int
    clip_key: str
    clip_type: str
    title: str
    file_path: str
    start_sec: float
    end_sec: float

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.start_sec, self.end_sec) or "00:00"


@dataclass(frozen=True)
class SearchResult:
    """Search result returned to the API layer."""

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
    source_start_sec: float
    source_end_sec: float
    tags: list[str] = field(default_factory=list)
    clips: list[ClipResult] = field(default_factory=list)
    unit_key: str = ""
    source_key: str = ""
    source_type: str = "video"
    unit_metadata: dict[str, Any] = field(default_factory=dict)
    source_metadata: dict[str, Any] = field(default_factory=dict)
    lifecycle_status: str = "generated"
    review_status: str = "generated"
    quality_score: float = 0.0
    publication_id: str = ""
    published_version: int | None = None
    publication_key: str = ""
    publication_batch_key: str = ""

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.source_start_sec, self.source_end_sec) or "00:00"


class KnowledgeLibraryUnavailableError(RuntimeError):
    """Raised when the lifecycle-owned async knowledge pool is unavailable."""


class KnowledgeLibraryBase:
    """Lifecycle-owned pool state shared by the knowledge library mixins."""

    embedding_generator: EmbeddingGenerator
    _pool: AsyncConnectionPool[AsyncConnection[TupleRow]] | None
    _owns_pool: bool
    _init_lock: asyncio.Lock

    def _require_pool(self) -> AsyncConnectionPool[AsyncConnection[TupleRow]]:
        if self._pool is None:
            raise KnowledgeLibraryUnavailableError(
                "KnowledgeLibrary is not initialized; use the FastAPI lifespan "
                "or an explicit test pool"
            )
        return self._pool
