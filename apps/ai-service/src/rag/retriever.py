"""Semantic retrieval module using pgvector."""

import os
from dataclasses import dataclass
from typing import Optional

import psycopg
from pgvector.psycopg import register_vector

from .embedding import EmbeddingGenerator, get_embedding_generator


@dataclass
class RetrievalResult:
    """Result from semantic retrieval."""

    id: int
    category: str
    title: str
    content: str
    similarity: float
    source_video: Optional[str] = None
    source_timestamp: Optional[str] = None


class SemanticRetriever:
    """Perform semantic search using pgvector cosine similarity."""

    def __init__(
        self,
        database_url: Optional[str] = None,
        embedding_generator: Optional[EmbeddingGenerator] = None,
    ):
        self.database_url = database_url or os.getenv("DATABASE_URL")
        self.embedding_generator = embedding_generator or get_embedding_generator()
        self._connection: Optional[psycopg.Connection] = None

    def _get_connection(self) -> psycopg.Connection:
        """Get or create database connection."""
        if self._connection is None or self._connection.closed:
            if not self.database_url:
                raise ValueError("DATABASE_URL is required")

            # Convert async URL format if needed
            url = self.database_url.replace("+asyncpg", "").replace("+psycopg", "")

            self._connection = psycopg.connect(url)
            register_vector(self._connection)

        return self._connection

    async def search(
        self,
        query: str,
        top_k: int = 10,
        category: Optional[str] = None,
    ) -> list[RetrievalResult]:
        """
        Perform semantic search.

        Args:
            query: The search query text.
            top_k: Number of results to return.
            category: Optional category filter.

        Returns:
            List of retrieval results with similarity scores.
        """
        # Generate query embedding
        query_embedding = await self.embedding_generator.generate(query)

        conn = self._get_connection()

        # Build query
        if category:
            sql = """
                SELECT id, category, title, content, source_video,
                       source_timestamp,
                       1 - (embedding <=> %s::vector) as similarity
                FROM knowledge_entries
                WHERE embedding IS NOT NULL AND category = %s
                ORDER BY embedding <=> %s::vector
                LIMIT %s
            """
            params = (query_embedding, category, query_embedding, top_k)
        else:
            sql = """
                SELECT id, category, title, content, source_video,
                       source_timestamp,
                       1 - (embedding <=> %s::vector) as similarity
                FROM knowledge_entries
                WHERE embedding IS NOT NULL
                ORDER BY embedding <=> %s::vector
                LIMIT %s
            """
            params = (query_embedding, query_embedding, top_k)

        cur = conn.cursor()
        try:
            cur.execute(sql, params)
            rows = cur.fetchall()
        finally:
            cur.close()

        results = []
        for row in rows:
            results.append(
                RetrievalResult(
                    id=row[0],
                    category=row[1],
                    title=row[2],
                    content=row[3],
                    similarity=row[6],
                    source_video=row[4],
                    source_timestamp=row[5],
                )
            )

        return results

    async def close(self):
        """Close database connection."""
        if self._connection and not self._connection.closed:
            self._connection.close()


# Singleton instance
_default_retriever: Optional[SemanticRetriever] = None


def get_semantic_retriever() -> SemanticRetriever:
    """Get or create the default semantic retriever."""
    global _default_retriever
    if _default_retriever is None:
        _default_retriever = SemanticRetriever()
    return _default_retriever
