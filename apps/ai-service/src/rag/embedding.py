"""Embedding generation module for RAG pipeline."""

import os
from typing import Optional

from openai import AsyncOpenAI


class EmbeddingGenerator:
    """Generate text embeddings using OpenAI API or local models."""

    def __init__(
        self,
        model: str = "text-embedding-3-small",
        dimension: int = 1536,
        api_key: Optional[str] = None,
    ):
        self.model = model
        self.dimension = dimension
        self.api_key = api_key or os.getenv("OPENAI_API_KEY")
        self._client: Optional[AsyncOpenAI] = None

    @property
    def client(self) -> AsyncOpenAI:
        """Lazy initialization of OpenAI client."""
        if self._client is None:
            if not self.api_key:
                raise ValueError("OPENAI_API_KEY is required for OpenAI embeddings")
            self._client = AsyncOpenAI(api_key=self.api_key)
        return self._client

    async def generate(self, text: str) -> list[float]:
        """
        Generate embedding for a single text.

        Args:
            text: The text to embed.

        Returns:
            A list of floats representing the embedding vector.
        """
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

        # OpenAI API supports batch requests
        response = await self.client.embeddings.create(
            model=self.model,
            input=texts,
            dimensions=self.dimension,
        )

        # Extract embeddings from response
        embeddings = [item.embedding for item in response.data]

        # Validate dimensions
        for i, emb in enumerate(embeddings):
            if len(emb) != self.dimension:
                raise ValueError(
                    f"Embedding dimension mismatch for text {i}: "
                    f"expected {self.dimension}, got {len(emb)}"
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
                    # Exponential backoff
                    import asyncio
                    await asyncio.sleep(2 ** attempt)

        raise last_error


# Singleton instance
_default_generator: Optional[EmbeddingGenerator] = None


def get_embedding_generator() -> EmbeddingGenerator:
    """Get or create the default embedding generator."""
    global _default_generator
    if _default_generator is None:
        _default_generator = EmbeddingGenerator()
    return _default_generator
