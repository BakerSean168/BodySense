"""Unit tests for retriever module."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from src.rag.retriever import RetrievalResult, SemanticRetriever


class MockAsyncContextManager:
    """Helper class for mocking async context managers."""

    def __init__(self, return_value=None):
        self.return_value = return_value

    async def __aenter__(self):
        return self.return_value

    async def __aexit__(self, *args):
        pass


def create_mock_connection(mock_cursor):
    """Create a mock database connection that returns a mock cursor."""
    mock_conn = AsyncMock()
    mock_conn.closed = False

    # Make conn.cursor() return the mock_cursor directly (not as a coroutine)
    mock_conn.cursor = MagicMock(return_value=mock_cursor)

    return mock_conn


class TestSemanticRetriever:
    """Tests for SemanticRetriever class."""

    def test_retrieval_result_creation(self):
        """Test RetrievalResult dataclass creation."""
        result = RetrievalResult(
            id=1,
            category="posture",
            title="Test Title",
            content="Test content",
            similarity=0.95,
            source_video="https://example.com/video",
            source_timestamp="10:30",
        )

        assert result.id == 1
        assert result.category == "posture"
        assert result.title == "Test Title"
        assert result.content == "Test content"
        assert result.similarity == 0.95
        assert result.source_video == "https://example.com/video"
        assert result.source_timestamp == "10:30"

    def test_retrieval_result_optional_fields(self):
        """Test RetrievalResult with optional fields as None."""
        result = RetrievalResult(
            id=1,
            category="posture",
            title="Test Title",
            content="Test content",
            similarity=0.95,
        )

        assert result.source_video is None
        assert result.source_timestamp is None

    @pytest.mark.asyncio
    async def test_search_success(self):
        """Test successful semantic search."""
        retriever = SemanticRetriever(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
        )

        # Mock embedding generator
        mock_embedding = [0.1] * 1536
        retriever.embedding_generator.generate = AsyncMock(return_value=mock_embedding)

        # Mock database connection and cursor
        mock_cursor = AsyncMock()
        mock_cursor.fetchall = AsyncMock(return_value=[
            (1, "posture", "Title 1", "Content 1", None, None, 0.95),
            (2, "exercise", "Title 2", "Content 2", None, None, 0.85),
        ])

        mock_conn = create_mock_connection(mock_cursor)
        retriever._connection = mock_conn

        results = await retriever.search("test query", top_k=2)

        assert len(results) == 2
        assert results[0].id == 1
        assert results[0].similarity == 0.95
        assert results[1].id == 2
        assert results[1].similarity == 0.85

    @pytest.mark.asyncio
    async def test_search_with_category_filter(self):
        """Test semantic search with category filter."""
        retriever = SemanticRetriever(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
        )

        # Mock embedding generator
        mock_embedding = [0.1] * 1536
        retriever.embedding_generator.generate = AsyncMock(return_value=mock_embedding)

        # Mock database
        mock_cursor = AsyncMock()
        mock_cursor.fetchall = AsyncMock(return_value=[
            (1, "posture", "Title 1", "Content 1", None, None, 0.95),
        ])

        mock_conn = create_mock_connection(mock_cursor)
        retriever._connection = mock_conn

        results = await retriever.search("test query", top_k=10, category="posture")

        assert len(results) == 1
        assert results[0].category == "posture"

    @pytest.mark.asyncio
    async def test_search_empty_results(self):
        """Test search with no results."""
        retriever = SemanticRetriever(
            database_url="postgresql://test:test@localhost/test",
            embedding_generator=MagicMock(),
        )

        # Mock embedding generator
        mock_embedding = [0.1] * 1536
        retriever.embedding_generator.generate = AsyncMock(return_value=mock_embedding)

        # Mock database
        mock_cursor = AsyncMock()
        mock_cursor.fetchall = AsyncMock(return_value=[])

        mock_conn = create_mock_connection(mock_cursor)
        retriever._connection = mock_conn

        results = await retriever.search("test query", top_k=10)

        assert len(results) == 0
