"""Embedding generation module for RAG pipeline."""

import os
from typing import Optional

from openai import AsyncOpenAI


class EmbeddingGenerator:
    """Generate text embeddings using OpenAI-compatible API or local models."""

    def __init__(
        self,
        model: Optional[str] = None,
        dimension: int = 384,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
    ):
        self.provider = os.getenv("EMBEDDING_PROVIDER", "local")
        self.model = model or os.getenv(
            "EMBEDDING_MODEL", "all-MiniLM-L6-v2"
        )
        self.dimension = dimension
        self.api_key = (
            api_key
            or os.getenv("EMBEDDING_API_KEY")
            or os.getenv("OPENAI_API_KEY")
        )
        self.base_url = base_url or os.getenv("EMBEDDING_BASE_URL")
        self._client: Optional[AsyncOpenAI] = None
        self._local_model = None

    def _get_local_model(self):
        """Get or create local sentence-transformers model."""
        if self._local_model is None:
            from sentence_transformers import SentenceTransformer

            self._local_model = SentenceTransformer(self.model)
            # Update dimension based on model output
            test_embedding = self._local_model.encode(["test"])
            self.dimension = len(test_embedding[0])
        return self._local_model

    @property
    def client(self) -> AsyncOpenAI:
        """Lazy initialization of OpenAI-compatible client."""
        if self._client is None:
            if not self.api_key:
                raise ValueError(
                    "EMBEDDING_API_KEY or OPENAI_API_KEY is required"
                )
            kwargs = {"api_key": self.api_key}
            if self.base_url:
                kwargs["base_url"] = self.base_url
            self._client = AsyncOpenAI(**kwargs)
        return self._client

    async def generate(self, text: str) -> list[float]:
        """
        Generate embedding for a single text.

        Args:
            text: The text to embed.

        Returns:
            A list of floats representing the embedding vector.
        """
        if self.provider == "local":
            model = self._get_local_model()
            embedding = model.encode([text])[0]
            return embedding.tolist()

        # Use OpenAI-compatible API
        embeddings = await self.generate_batch([text])
        return embeddings[0]

    async def generate_batch(self, texts: list[str]) -> list[list[float]]:
        """
        Generate embeddings for multiple texts.

        Args:
            texts: List of texts to embed.

        Returns:
            List of embedding vectors.
        """
        if not texts:
            return []

        if self.provider == "local":
            model = self._get_local_model()
            embeddings = model.encode(texts)
            return [emb.tolist() for emb in embeddings]

        # Use OpenAI-compatible API
        response = await self.client.embeddings.create(
            model=self.model,
            input=texts,
        )

        # Extract embeddings from response
        embeddings = [item.embedding for item in response.data]

        # Validate dimension
        if embeddings and len(embeddings[0]) != self.dimension:
            raise ValueError(
                f"Embedding dimension mismatch: expected {self.dimension}, "
                f"got {len(embeddings[0])}"
            )

        return embeddings

    async def generate_with_retry(
        self,
        text: str,
        max_retries: int = 3,
    ) -> list[float]:
        """
        Generate embedding with retry logic.

        Args:
            text: The text to embed.
            max_retries: Maximum number of retry attempts.

        Returns:
            Embedding vector.
        """
        last_error = None
        for attempt in range(max_retries):
            try:
                return await self.generate(text)
            except Exception as e:
                last_error = e
                if attempt < max_retries - 1:
                    import asyncio

                    await asyncio.sleep(2**attempt)

        raise last_error


# Singleton instance
_default_generator: Optional[EmbeddingGenerator] = None


def get_embedding_generator() -> EmbeddingGenerator:
    """Get or create the default embedding generator."""
    global _default_generator
    if _default_generator is None:
        _default_generator = EmbeddingGenerator()
    return _default_generator
