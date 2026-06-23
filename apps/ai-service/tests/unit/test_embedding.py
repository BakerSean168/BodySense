"""Unit tests for embedding module."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from src.rag.embedding import EmbeddingGenerator


class TestEmbeddingGenerator:
    """Tests for EmbeddingGenerator class."""

    def test_init_default(self):
        """Test default initialization with OpenAI provider."""
        with patch.dict("os.environ", {
            "EMBEDDING_PROVIDER": "openai",
            "EMBEDDING_MODEL": "all-MiniLM-L6-v2",
        }):
            gen = EmbeddingGenerator(api_key="test-key")
            assert gen.model == "all-MiniLM-L6-v2"
            assert gen.dimension == 1536
            assert gen.api_key == "test-key"
            assert gen.provider == "openai"

    def test_init_custom(self):
        """Test custom initialization."""
        with patch.dict("os.environ", {
            "EMBEDDING_DIMENSIONS": "768",
        }):
            gen = EmbeddingGenerator(
                model="custom-model",
                dimension=768,
                api_key="test-key",
            )
            assert gen.model == "custom-model"
            assert gen.dimension == 768

    @pytest.mark.asyncio
    async def test_generate_single(self):
        """Test single text embedding generation via OpenAI API."""
        gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
        gen.provider = "openai"

        # Mock the client
        mock_embedding = [0.1] * 1536
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=mock_embedding)]

        gen._client = MagicMock()
        gen._client.embeddings.create = AsyncMock(return_value=mock_response)

        result = await gen.generate("test text")

        assert len(result) == 1536
        assert result == mock_embedding

    @pytest.mark.asyncio
    async def test_generate_batch(self):
        """Test batch embedding generation."""
        gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
        gen.provider = "openai"

        # Mock the client
        mock_embeddings = [[0.1] * 1536, [0.2] * 1536]
        mock_response = MagicMock()
        mock_response.data = [
            MagicMock(embedding=mock_embeddings[0]),
            MagicMock(embedding=mock_embeddings[1]),
        ]

        gen._client = MagicMock()
        gen._client.embeddings.create = AsyncMock(return_value=mock_response)

        result = await gen.generate_batch(["text1", "text2"])

        assert len(result) == 2
        assert result[0] == mock_embeddings[0]
        assert result[1] == mock_embeddings[1]

    @pytest.mark.asyncio
    async def test_generate_empty_batch(self):
        """Test empty batch returns empty list."""
        gen = EmbeddingGenerator(api_key="test-key")
        result = await gen.generate_batch([])
        assert result == []

    @pytest.mark.asyncio
    async def test_generate_dimension_mismatch(self):
        """Test dimension mismatch raises error."""
        gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
        gen.provider = "openai"

        # Mock response with wrong dimension
        mock_embedding = [0.1] * 768  # Wrong dimension
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=mock_embedding)]

        gen._client = MagicMock()
        gen._client.embeddings.create = AsyncMock(return_value=mock_response)

        with pytest.raises(ValueError, match="Embedding dimension mismatch"):
            await gen.generate("test text")

    @pytest.mark.asyncio
    async def test_generate_with_retry(self):
        """Test retry logic on failure."""
        gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
        gen.provider = "openai"

        # Mock client to fail twice then succeed
        mock_embedding = [0.1] * 1536
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=mock_embedding)]

        gen._client = MagicMock()
        gen._client.embeddings.create = AsyncMock(
            side_effect=[Exception("API Error"), Exception("API Error"), mock_response]
        )

        result = await gen.generate_with_retry("test text", max_retries=3)

        assert len(result) == 1536
        assert gen._client.embeddings.create.call_count == 3

    @pytest.mark.asyncio
    async def test_generate_with_retry_exhausted(self):
        """Test retry exhaustion raises last error."""
        gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
        gen.provider = "openai"

        gen._client = MagicMock()
        gen._client.embeddings.create = AsyncMock(
            side_effect=Exception("API Error")
        )

        with pytest.raises(Exception, match="API Error"):
            await gen.generate_with_retry("test text", max_retries=2)

        assert gen._client.embeddings.create.call_count == 2
