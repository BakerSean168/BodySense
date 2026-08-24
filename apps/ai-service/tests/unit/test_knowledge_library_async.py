"""Concurrency and transaction tests for the async KnowledgeLibrary boundary."""

from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from src.rag.knowledge_library import (
    KNOWLEDGE_CONNECT_TIMEOUT_SECONDS,
    KnowledgeLibrary,
    KnowledgeLibraryUnavailableError,
)
from src.rag.knowledge_pack import GeneratedKnowledgePack, SourceVideoMetadata


class FakeLifecyclePool:
    instances: list["FakeLifecyclePool"] = []

    def __init__(self, *args, **kwargs):
        self.open_calls = 0
        self.close_calls = 0
        self.fail_open = kwargs.pop("fail_open", False)
        FakeLifecyclePool.instances.append(self)

    async def open(self, *, wait: bool, timeout: float) -> None:
        self.open_calls += 1
        assert wait is True
        assert timeout == KNOWLEDGE_CONNECT_TIMEOUT_SECONDS
        await asyncio.sleep(0)

    async def close(self) -> None:
        self.close_calls += 1


@pytest.mark.asyncio
async def test_pool_initialization_is_concurrency_safe_and_close_is_owned() -> None:
    FakeLifecyclePool.instances.clear()
    with patch(
        "src.rag.knowledge_library.AsyncConnectionPool",
        FakeLifecyclePool,
    ):
        library = KnowledgeLibrary(database_url="postgresql://test")
        await asyncio.gather(*(library.initialize() for _ in range(16)))
        assert len(FakeLifecyclePool.instances) == 1
        assert FakeLifecyclePool.instances[0].open_calls == 1
        await library.close()
        assert FakeLifecyclePool.instances[0].close_calls == 1


@pytest.mark.asyncio
async def test_pool_startup_failure_is_bounded_and_closed() -> None:
    class FailingPool(FakeLifecyclePool):
        async def open(self, *, wait: bool, timeout: float) -> None:
            await super().open(wait=wait, timeout=timeout)
            raise TimeoutError("database unreachable")

    FailingPool.instances.clear()
    with patch("src.rag.knowledge_library.AsyncConnectionPool", FailingPool):
        library = KnowledgeLibrary(database_url="postgresql://test")
        with pytest.raises(KnowledgeLibraryUnavailableError, match="bounded startup"):
            await library.initialize()
        assert FailingPool.instances[0].close_calls == 1


class FakeTransaction:
    def __init__(self) -> None:
        self.exit_exception_type = None

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        self.exit_exception_type = exc_type
        return False


class FailingCursor:
    def __init__(self) -> None:
        self.execute_calls = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    async def execute(self, query, params=None) -> None:
        self.execute_calls += 1
        if self.execute_calls == 2:
            raise RuntimeError("insert exploded")

    async def fetchone(self):
        if self.execute_calls == 1:
            return (
                41,
                "registered",
                "video",
                "Source",
                "Author",
                "posture",
                "Posture",
                "zh",
                "owned",
                "a" * 64,
                {"origin": "test"},
                "00000000-0000-0000-0000-000000000001",
                object(),
            )
        return None


class FailingConnection:
    def __init__(self) -> None:
        self.tx = FakeTransaction()
        self.cursor_instance = FailingCursor()

    def transaction(self):
        return self.tx

    def cursor(self):
        return self.cursor_instance


class InjectedPool:
    def __init__(self, conn: FailingConnection) -> None:
        self.conn = conn
        self.closed = False

    @asynccontextmanager
    async def connection(self):
        yield self.conn

    async def close(self) -> None:
        self.closed = True


@pytest.mark.asyncio
async def test_ingest_transaction_rolls_back_on_write_failure() -> None:
    conn = FailingConnection()
    pool = InjectedPool(conn)
    embedder = MagicMock()
    embedder.generate_batch = AsyncMock(return_value=[])
    embedder.identity.return_value = {
        "provider": "hashing",
        "model": "bodysense-hashing-ngram",
        "dimension": 1536,
        "revision": "sha256-char-word-ngram-v1",
        "fingerprint": "a" * 64,
    }
    library = KnowledgeLibrary(
        database_url="postgresql://test",
        embedding_generator=embedder,
        pool=pool,  # type: ignore[arg-type]
        owns_pool=False,
    )
    pack = GeneratedKnowledgePack(
        source=SourceVideoMetadata(
            source_key="source-1",
            source_type="video",
            title="Source",
            author="Author",
            problem_slug="posture",
            problem_display_name="Posture",
            original_file_path="/tmp/source.mp4",
        ),
        artifact_dir="/tmp/artifacts",
        transcript_segments=[],
        units=[],
        clips=[],
    )

    with pytest.raises(RuntimeError, match="insert exploded"):
        await library.ingest_generated_pack(pack)

    assert conn.tx.exit_exception_type is RuntimeError
    # The injected pool is test-owned and is not implicitly closed by the library.
    await library.close()
    assert pool.closed is False


class PublishedSourceCursor:
    def __init__(self) -> None:
        self.query_index = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    async def execute(self, query, params=None) -> None:
        self.query_index += 1

    async def fetchone(self):
        if self.query_index == 1:
            return (
                77,
                "ingested",
                "thought_forest_note",
                "Published source",
                "Thought Forest",
                "pain",
                "Pain",
                "zh",
                "citation_only",
                "b" * 64,
                {"origin": "test"},
                "00000000-0000-0000-0000-000000000001",
                object(),
            )
        if self.query_index == 2:
            return (1,)
        return None


class PublishedSourceConnection:
    def __init__(self) -> None:
        self.tx = FakeTransaction()
        self.cursor_instance = PublishedSourceCursor()

    def transaction(self):
        return self.tx

    def cursor(self):
        return self.cursor_instance


@pytest.mark.asyncio
async def test_overwrite_rejects_source_with_published_or_publication_linked_units() -> None:
    conn = PublishedSourceConnection()
    pool = InjectedPool(conn)  # type: ignore[arg-type]
    embedder = MagicMock()
    embedder.generate_batch = AsyncMock(return_value=[])
    embedder.identity.return_value = {
        "provider": "hashing",
        "model": "bodysense-hashing-ngram",
        "dimension": 1536,
        "revision": "sha256-char-word-ngram-v1",
        "fingerprint": "a" * 64,
    }
    library = KnowledgeLibrary(
        database_url="postgresql://test",
        embedding_generator=embedder,
        pool=pool,  # type: ignore[arg-type]
        owns_pool=False,
    )
    pack = GeneratedKnowledgePack(
        source=SourceVideoMetadata(
            source_key="source-published",
            source_type="thought_forest_note",
            title="Published source",
            author="Thought Forest",
            problem_slug="pain",
            problem_display_name="Pain",
            original_file_path="z/pain.md",
        ),
        artifact_dir="thought-forest://published",
        transcript_segments=[],
        units=[],
        clips=[],
    )

    with pytest.raises(RuntimeError, match="cannot overwrite a knowledge source"):
        await library.ingest_generated_pack(pack, overwrite_source=True)

    assert conn.tx.exit_exception_type is RuntimeError


class BatchPreflightCursor:
    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False

    async def execute(self, query, params=None) -> None:
        return None

    async def fetchall(self):
        return [("thought-forest:z/pain.md",)]


class BatchPreflightConnection:
    def cursor(self):
        return BatchPreflightCursor()


class BatchPreflightPool:
    @asynccontextmanager
    async def connection(self):
        yield BatchPreflightConnection()


@pytest.mark.asyncio
async def test_batch_overwrite_preflight_fails_before_any_source_write() -> None:
    library = KnowledgeLibrary(
        database_url="postgresql://test",
        embedding_generator=AsyncMock(),
        pool=BatchPreflightPool(),  # type: ignore[arg-type]
        owns_pool=False,
    )

    with pytest.raises(RuntimeError, match="thought-forest:z/pain.md"):
        await library.assert_sources_overwritable(
            ["thought-forest:z/glute.md", "thought-forest:z/pain.md"]
        )
