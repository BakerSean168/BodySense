"""Persistent knowledge library built around video sources, units, and clips."""

from __future__ import annotations

import asyncio
import logging
import os
from contextlib import asynccontextmanager
from typing import AsyncIterator, Optional

from pgvector.psycopg import register_vector_async
from psycopg import AsyncConnection
from psycopg.rows import TupleRow
from psycopg_pool import AsyncConnectionPool

from .embedding import EmbeddingGenerator, get_embedding_generator
from .knowledge_library_admin import KnowledgeAdminMixin
from .knowledge_library_base import (
    ClipResult,
    KnowledgeLibraryUnavailableError,
    SearchResult,
)
from .knowledge_library_ingest import KnowledgeIngestMixin
from .knowledge_library_search import KnowledgeSearchMixin

__all__ = [
    "KNOWLEDGE_CONNECT_TIMEOUT_SECONDS",
    "KNOWLEDGE_POOL_MAX_SIZE",
    "KNOWLEDGE_POOL_MIN_SIZE",
    "ClipResult",
    "KnowledgeLibrary",
    "KnowledgeLibraryUnavailableError",
    "SearchResult",
    "get_knowledge_library",
    "initialize_knowledge_library",
    "knowledge_library_lifespan",
    "shutdown_knowledge_library",
]

logger = logging.getLogger(__name__)


KNOWLEDGE_CONNECT_TIMEOUT_SECONDS = 5.0
KNOWLEDGE_POOL_MIN_SIZE = 1
KNOWLEDGE_POOL_MAX_SIZE = 8


async def _configure_vector_connection(conn: AsyncConnection[TupleRow]) -> None:
    await register_vector_async(conn)


class KnowledgeLibrary(KnowledgeSearchMixin, KnowledgeIngestMixin, KnowledgeAdminMixin):
    """Persist and search structured knowledge with lifecycle-owned async I/O."""

    def __init__(
        self,
        database_url: Optional[str] = None,
        embedding_generator: Optional[EmbeddingGenerator] = None,
        *,
        pool: AsyncConnectionPool[AsyncConnection[TupleRow]] | None = None,
        owns_pool: bool | None = None,
    ):
        self.database_url = database_url or self._build_database_url()
        self.embedding_generator = embedding_generator or get_embedding_generator()
        self._pool = pool
        self._owns_pool = (pool is None) if owns_pool is None else owns_pool
        self._init_lock = asyncio.Lock()

    def _build_database_url(self) -> str:
        env_url = os.getenv("DATABASE_URL")
        if env_url:
            return env_url.replace("+asyncpg", "").replace("+psycopg", "")

        host = os.getenv("DB_HOST", "127.0.0.1")
        port = os.getenv("DB_PORT", "5432")
        name = os.getenv("DB_NAME", "bodysense")
        user = os.getenv("DB_USER", "bodysense")
        password = os.getenv("DB_PASSWORD")
        if password is None:
            password = "bodysense123"
            logger.warning(
                "DB_PASSWORD not set, using default password. Set DB_PASSWORD in your .env file."
            )
        return f"postgresql://{user}:{password}@{host}:{port}/{name}"

    async def initialize(self) -> "KnowledgeLibrary":
        """Open the bounded async pool once; startup failures surface promptly."""
        if self._pool is not None:
            return self
        async with self._init_lock:
            if self._pool is not None:
                return self
            pool: AsyncConnectionPool[AsyncConnection[TupleRow]] = AsyncConnectionPool(
                self.database_url,
                min_size=int(os.getenv("KNOWLEDGE_POOL_MIN_SIZE", str(KNOWLEDGE_POOL_MIN_SIZE))),
                max_size=int(os.getenv("KNOWLEDGE_POOL_MAX_SIZE", str(KNOWLEDGE_POOL_MAX_SIZE))),
                open=False,
                configure=_configure_vector_connection,
            )
            try:
                await pool.open(wait=True, timeout=KNOWLEDGE_CONNECT_TIMEOUT_SECONDS)
            except Exception as exc:
                await pool.close()
                raise KnowledgeLibraryUnavailableError(
                    "knowledge Postgres pool unavailable during bounded startup"
                ) from exc
            self._pool = pool
            self._owns_pool = True
            logger.info("Initialized async KnowledgeLibrary Postgres pool")
        return self

    async def close(self) -> None:
        pool = self._pool
        self._pool = None
        if pool is not None and self._owns_pool:
            await pool.close()


_default_knowledge_library: Optional[KnowledgeLibrary] = None


def get_knowledge_library() -> KnowledgeLibrary:
    """Return the lifecycle-owned instance; this function never performs I/O."""
    global _default_knowledge_library
    if _default_knowledge_library is None:
        _default_knowledge_library = KnowledgeLibrary()
    return _default_knowledge_library


async def initialize_knowledge_library() -> KnowledgeLibrary:
    library = get_knowledge_library()
    await library.initialize()
    return library


async def shutdown_knowledge_library() -> None:
    global _default_knowledge_library
    library = _default_knowledge_library
    _default_knowledge_library = None
    if library is not None:
        await library.close()


@asynccontextmanager
async def knowledge_library_lifespan() -> AsyncIterator[None]:
    await initialize_knowledge_library()
    try:
        yield
    finally:
        await shutdown_knowledge_library()
