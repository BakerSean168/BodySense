"""Unit tests for knowledge_base module."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from src.rag.knowledge_base import KnowledgeBase, KnowledgeEntryData
from src.rag.retriever import RetrievalResult


def create_mock_connection(mock_cursor):
    """Create a mock database connection that returns a mock cursor."""
    mock_conn = MagicMock()
    mock_conn.closed = False
    mock_conn.cursor.return_value = mock_cursor
    return mock_conn


class TestKnowledgeBase:
    """Tests for KnowledgeBase class."""

    def test_knowledge_entry_data_creation(self):
        """Test KnowledgeEntryData dataclass creation."""
        entry = KnowledgeEntryData(
            category="posture",
            title="Test Title",
            content="Test content",
            source_video="https://example.com/video",
            source_timestamp="10:30",
        )

        assert entry.category == "posture"
        assert entry.title == "Test Title"
        assert entry.content == "Test content"
        assert entry.source_video == "https://example.com/video"
        assert entry.source_timestamp == "10:30"

    def test_knowledge_entry_data_optional_fields(self):
        """Test KnowledgeEntryData with optional fields as None."""
        entry = KnowledgeEntryData(
            category="posture",
            title="Test Title",
            content="Test content",
        )

        assert entry.source_video is None
        assert entry.source_timestamp is None

    @pytest.mark.asyncio
    async def test_add_entry_success(self):
        """Test successful entry addition."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock embedding generator
        mock_embedding = [0.1] * 1536
        kb.embedding_generator.generate = AsyncMock(return_value=mock_embedding)

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.fetchone.return_value = (1,)

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        entry = KnowledgeEntryData(
            category="posture",
            title="Test Title",
            content="Test content",
        )

        entry_id = await kb.add_entry(entry)

        assert entry_id == 1
        kb.embedding_generator.generate.assert_called_once_with("Test content")

    @pytest.mark.asyncio
    async def test_add_entries_batch_success(self):
        """Test successful batch entry addition."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock embedding generator
        mock_embeddings = [[0.1] * 1536, [0.2] * 1536]
        kb.embedding_generator.generate_batch = AsyncMock(return_value=mock_embeddings)

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.fetchone.side_effect = [(1,), (2,)]

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        entries = [
            KnowledgeEntryData(category="posture", title="Title 1", content="Content 1"),
            KnowledgeEntryData(category="exercise", title="Title 2", content="Content 2"),
        ]

        ids = await kb.add_entries_batch(entries)

        assert ids == [1, 2]
        kb.embedding_generator.generate_batch.assert_called_once()

    @pytest.mark.asyncio
    async def test_add_entries_batch_empty(self):
        """Test batch addition with empty list."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        ids = await kb.add_entries_batch([])

        assert ids == []

    @pytest.mark.asyncio
    async def test_search_success(self):
        """Test successful end-to-end search."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock retriever
        mock_candidates = [
            RetrievalResult(id=1, category="test", title="T1", content="C1", similarity=0.9),
            RetrievalResult(id=2, category="test", title="T2", content="C2", similarity=0.8),
        ]
        kb.retriever.search = AsyncMock(return_value=mock_candidates)

        # Mock reranker
        mock_reranked = [
            RetrievalResult(id=2, category="test", title="T2", content="C2", similarity=0.8),
            RetrievalResult(id=1, category="test", title="T1", content="C1", similarity=0.9),
        ]
        kb.reranker.rerank = AsyncMock(return_value=mock_reranked)

        results = await kb.search("test query", top_k=10, top_n=2)

        assert len(results) == 2
        assert results[0].id == 2
        assert results[1].id == 1
        kb.retriever.search.assert_called_once_with(
            query="test query",
            top_k=10,
            category=None,
        )
        kb.reranker.rerank.assert_called_once_with(
            query="test query",
            candidates=mock_candidates,
            top_n=2,
        )

    @pytest.mark.asyncio
    async def test_get_entry_found(self):
        """Test getting an entry that exists."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.fetchone.return_value = (
            1, "posture", "Title", "Content", None, None,
        )

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        entry = await kb.get_entry(1)

        assert entry is not None
        assert entry.id == 1
        assert entry.category == "posture"

    @pytest.mark.asyncio
    async def test_get_entry_not_found(self):
        """Test getting an entry that doesn't exist."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.fetchone.return_value = None

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        entry = await kb.get_entry(999)

        assert entry is None

    @pytest.mark.asyncio
    async def test_delete_entry_success(self):
        """Test successful entry deletion."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.rowcount = 1

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        result = await kb.delete_entry(1)

        assert result is True

    @pytest.mark.asyncio
    async def test_delete_entry_not_found(self):
        """Test deleting a non-existent entry."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.rowcount = 0

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        result = await kb.delete_entry(999)

        assert result is False

    @pytest.mark.asyncio
    async def test_count(self):
        """Test counting entries."""
        kb = KnowledgeBase(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
            retriever=MagicMock(),
            reranker=MagicMock(),
        )

        # Mock database — psycopg cursor is synchronous
        mock_cursor = MagicMock()
        mock_cursor.fetchone.return_value = (42,)

        mock_conn = create_mock_connection(mock_cursor)
        kb._connection = mock_conn

        count = await kb.count()

        assert count == 42
