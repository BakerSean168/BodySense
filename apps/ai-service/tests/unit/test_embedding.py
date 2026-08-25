"""Unit tests for embedding module."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from src.rag.embedding import EmbeddingGenerator


class TestEmbeddingGenerator:
    """Tests for EmbeddingGenerator class."""

    def test_init_default(self):
        """Test default initialization with OpenAI provider."""
        with patch.dict(
            "os.environ",
            {
                "EMBEDDING_PROVIDER": "openai",
                "EMBEDDING_MODEL": "all-MiniLM-L6-v2",
            },
        ):
            gen = EmbeddingGenerator(api_key="test-key")
            assert gen.model == "all-MiniLM-L6-v2"
            assert gen.dimension == 1536
            assert gen.api_key == "test-key"
            assert gen.provider == "openai"

    def test_init_custom(self):
        """Test custom initialization."""
        with patch.dict(
            "os.environ",
            {
                "EMBEDDING_DIMENSIONS": "768",
            },
        ):
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
        gen._client.embeddings.create = AsyncMock(side_effect=Exception("API Error"))

        with pytest.raises(Exception, match="API Error"):
            await gen.generate_with_retry("test text", max_retries=2)

        assert gen._client.embeddings.create.call_count == 2


def test_embedding_identity_is_stable_and_revision_bound(monkeypatch):
    monkeypatch.setenv("EMBEDDING_PROVIDER", "hashing")
    gen = EmbeddingGenerator(dimension=1536)
    identity = gen.identity()
    assert identity["provider"] == "hashing"
    assert identity["model"] == "bodysense-hashing-ngram"
    assert identity["revision"] == "sha256-char-word-ngram-v1"
    assert identity["dimension"] == 1536
    assert len(str(identity["fingerprint"])) == 64


def test_semantic_embedding_identity_uses_explicit_revision(monkeypatch):
    monkeypatch.setenv("EMBEDDING_PROVIDER", "openai")
    monkeypatch.setenv("EMBEDDING_MODEL", "text-embedding-model")
    monkeypatch.setenv("EMBEDDING_REVISION", "2026-08-25")
    gen = EmbeddingGenerator(api_key="test-key", dimension=1536)
    identity = gen.identity()
    assert identity["revision"] == "2026-08-25"
    assert identity["model"] == "text-embedding-model"


@pytest.mark.asyncio
async def test_local_transformer_encode_does_not_block_event_loop():
    """A sleeping stand-in for CPU-heavy encode must run on a worker thread."""
    import asyncio
    import time

    class Vector(list):
        def tolist(self):
            return list(self)

    class BlockingModel:
        def encode(self, texts):
            time.sleep(0.06)
            return [Vector([1.0, 2.0, 3.0]) for _ in texts]

    gen = EmbeddingGenerator(dimension=3)
    gen.provider = "local_transformer"
    gen._load_local_model_sync = lambda: (BlockingModel(), 3)  # type: ignore[method-assign]

    task = asyncio.create_task(gen.generate("heartbeat"))
    heartbeats = 0
    while not task.done():
        await asyncio.sleep(0.005)
        heartbeats += 1
    result = await task

    assert heartbeats >= 3
    assert result == [1.0, 2.0, 3.0]
    assert gen.dimension == 3


@pytest.mark.asyncio
async def test_local_transformer_concurrency_is_bounded(monkeypatch):
    import asyncio
    import threading
    import time

    monkeypatch.setenv("LOCAL_EMBEDDING_MAX_CONCURRENCY", "2")

    class Vector(list):
        def tolist(self):
            return list(self)

    class BlockingModel:
        def __init__(self):
            self.lock = threading.Lock()
            self.active = 0
            self.max_active = 0

        def encode(self, texts):
            with self.lock:
                self.active += 1
                self.max_active = max(self.max_active, self.active)
            try:
                time.sleep(0.03)
                return [Vector([0.1, 0.2]) for _ in texts]
            finally:
                with self.lock:
                    self.active -= 1

    model = BlockingModel()
    gen = EmbeddingGenerator(dimension=2)
    gen.provider = "local_transformer"
    gen._load_local_model_sync = lambda: (model, 2)  # type: ignore[method-assign]

    results = await asyncio.gather(*(gen.generate(f"text-{i}") for i in range(8)))
    assert all(result == [0.1, 0.2] for result in results)
    assert model.max_active <= 2
