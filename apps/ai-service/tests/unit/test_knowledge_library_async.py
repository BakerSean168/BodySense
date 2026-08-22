"""Concurrency and transaction tests for the async KnowledgeLibrary boundary."""

from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager
from unittest.mock import AsyncMock, patch

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
    embedder = AsyncMock()
    embedder.generate_batch.return_value = []
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
