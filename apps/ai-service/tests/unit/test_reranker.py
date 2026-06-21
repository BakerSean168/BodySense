"""Unit tests for reranker module."""

import pytest
from unittest.mock import AsyncMock, MagicMock

from src.rag.reranker import Reranker, RerankResult
from src.rag.retriever import RetrievalResult


class TestReranker:
    """Tests for Reranker class."""

    def _create_candidates(self, count: int) -> list[RetrievalResult]:
        """Helper to create test candidates."""
        return [
            RetrievalResult(
                id=i,
                category="test",
                title=f"Title {i}",
                content=f"Content {i}",
                similarity=0.9 - (i * 0.1),
            )
            for i in range(1, count + 1)
        ]

    def test_rerank_result_creation(self):
        """Test RerankResult dataclass creation."""
        result = RetrievalResult(
            id=1,
            category="test",
            title="Test",
            content="Content",
            similarity=0.9,
        )
        rerank_result = RerankResult(result=result, relevance_score=0.95)

        assert rerank_result.result == result
        assert rerank_result.relevance_score == 0.95

    @pytest.mark.asyncio
    async def test_rerank_empty_candidates(self):
        """Test rerank with empty candidates."""
        reranker = Reranker(api_key="test-key")

        result = await reranker.rerank("test query", [], top_n=3)

        assert result == []

    @pytest.mark.asyncio
    async def test_rerank_fewer_candidates_than_top_n(self):
        """Test rerank when fewer candidates than top_n."""
        reranker = Reranker(api_key="test-key")
        candidates = self._create_candidates(2)

        result = await reranker.rerank("test query", candidates, top_n=3)

        assert len(result) == 2
        assert result == candidates

    @pytest.mark.asyncio
    async def test_rerank_success(self):
        """Test successful reranking."""
        reranker = Reranker(api_key="test-key")
        candidates = self._create_candidates(5)

        # Mock LLM response
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "[3, 1, 5]"

        reranker._client = MagicMock()
        reranker._client.chat.completions.create = AsyncMock(return_value=mock_response)

        result = await reranker.rerank("test query", candidates, top_n=3)

        assert len(result) == 3
        assert result[0].id == 3
        assert result[1].id == 1
        assert result[2].id == 5

    @pytest.mark.asyncio
    async def test_rerank_with_json_markdown(self):
        """Test reranking with markdown-wrapped JSON response."""
        reranker = Reranker(api_key="test-key")
        candidates = self._create_candidates(5)

        # Mock LLM response with markdown code block
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = '```json\n[2, 4, 1]\n```'

        reranker._client = MagicMock()
        reranker._client.chat.completions.create = AsyncMock(return_value=mock_response)

        result = await reranker.rerank("test query", candidates, top_n=3)

        assert len(result) == 3
        assert result[0].id == 2
        assert result[1].id == 4
        assert result[2].id == 1

    @pytest.mark.asyncio
    async def test_rerank_failure_fallback(self):
        """Test fallback to original order on failure."""
        reranker = Reranker(api_key="test-key")
        candidates = self._create_candidates(5)

        # Mock LLM to raise exception
        reranker._client = MagicMock()
        reranker._client.chat.completions.create = AsyncMock(
            side_effect=Exception("API Error")
        )

        result = await reranker.rerank("test query", candidates, top_n=3)

        # Should fallback to original order
        assert len(result) == 3
        assert result[0].id == 1
        assert result[1].id == 2
        assert result[2].id == 3

    @pytest.mark.asyncio
    async def test_rerank_with_scores(self):
        """Test reranking with scores."""
        reranker = Reranker(api_key="test-key")
        candidates = self._create_candidates(5)

        # Mock LLM response
        mock_response = MagicMock()
        mock_response.choices = [MagicMock()]
        mock_response.choices[0].message.content = "[3, 1, 5]"

        reranker._client = MagicMock()
        reranker._client.chat.completions.create = AsyncMock(return_value=mock_response)

        result = await reranker.rerank_with_scores("test query", candidates, top_n=3)

        assert len(result) == 3
        assert isinstance(result[0], RerankResult)
        assert result[0].result.id == 3
        assert result[0].relevance_score == 1.0
        assert result[1].relevance_score == 0.9
        assert result[2].relevance_score == 0.8
