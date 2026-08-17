"""Embedding generation module for RAG pipeline."""

import hashlib
import math
import os
from typing import Optional

from openai import AsyncOpenAI


class EmbeddingGenerator:
    """Generate text embeddings using OpenAI-compatible API or local models."""

    def __init__(
        self,
        model: Optional[str] = None,
        dimension: int = 1536,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
    ):
        self.provider = os.getenv("EMBEDDING_PROVIDER", "hashing")
        self.model = model or os.getenv("EMBEDDING_MODEL", "paraphrase-multilingual-MiniLM-L12-v2")
        self.dimension = int(os.getenv("EMBEDDING_DIMENSIONS", str(dimension)))
        self.api_key = api_key or os.getenv("EMBEDDING_API_KEY") or os.getenv("OPENAI_API_KEY")
        self.base_url = base_url or os.getenv("EMBEDDING_BASE_URL")
        self._client: Optional[AsyncOpenAI] = None
        self._local_model = None

    def _get_local_model(self):
        """Get or create local sentence-transformers model."""
        if self._local_model is None:
            from sentence_transformers import (  # pyright: ignore[reportMissingImports]
                SentenceTransformer,
            )

            self._local_model = SentenceTransformer(self.model)
            # Update dimension based on model output
            test_embedding = self._local_model.encode(["test"])
            self.dimension = len(test_embedding[0])
        return self._local_model

    def _normalize_text(self, text: str) -> str:
        return " ".join(text.lower().split())

    def _hash_embedding(self, text: str) -> list[float]:
        """Generate a deterministic local embedding from char and word n-grams."""
        normalized = self._normalize_text(text)
        collapsed = normalized.replace(" ", "")
        features: list[tuple[str, float]] = []

        for token in normalized.split():
            if token:
                features.append((token, 1.0))

        for source_text, weight in ((collapsed, 1.0), (normalized, 0.6)):
            if not source_text:
                continue
            for n in (1, 2, 3):
                for index in range(max(0, len(source_text) - n + 1)):
                    gram = source_text[index : index + n]
                    if gram.strip():
                        features.append((gram, weight / n))

        vector = [0.0] * self.dimension
        for feature, weight in features:
            digest = hashlib.sha256(feature.encode("utf-8")).digest()
            bucket = int.from_bytes(digest[:8], "big") % self.dimension
            sign = 1.0 if digest[8] % 2 == 0 else -1.0
            vector[bucket] += sign * weight

        norm = math.sqrt(sum(value * value for value in vector))
        if norm == 0:
            return vector
        return [value / norm for value in vector]

    @property
    def client(self) -> AsyncOpenAI:
        """Lazy initialization of OpenAI-compatible client."""
        if self._client is None:
            if not self.api_key:
                raise ValueError("EMBEDDING_API_KEY or OPENAI_API_KEY is required")
            assert self.api_key is not None
            if self.base_url:
                self._client = AsyncOpenAI(api_key=self.api_key, base_url=self.base_url)
            else:
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
        if self.provider == "hashing":
            return self._hash_embedding(text)

        if self.provider == "local_transformer":
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

        if self.provider == "hashing":
            return [self._hash_embedding(text) for text in texts]

        if self.provider == "local_transformer":
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
                f"Embedding dimension mismatch: expected {self.dimension}, got {len(embeddings[0])}"
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
        last_error: Exception | None = None
        for attempt in range(max_retries):
            try:
                return await self.generate(text)
            except Exception as e:
                last_error = e
                if attempt < max_retries - 1:
                    import asyncio

                    await asyncio.sleep(2**attempt)

        if last_error is None:
            raise RuntimeError("Embedding generation failed after retries")
        raise last_error


# Singleton instance
_default_generator: Optional[EmbeddingGenerator] = None


def get_embedding_generator() -> EmbeddingGenerator:
    """Get or create the default embedding generator."""
    global _default_generator
    if _default_generator is None:
        _default_generator = EmbeddingGenerator()
    return _default_generator
