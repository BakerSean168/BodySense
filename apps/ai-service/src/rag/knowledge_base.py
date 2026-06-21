"""Knowledge base management module."""

import os
from dataclasses import dataclass
from typing import Optional

import psycopg
from pgvector.psycopg import register_vector

from .embedding import EmbeddingGenerator, get_embedding_generator
from .reranker import Reranker, get_reranker
from .retriever import RetrievalResult, SemanticRetriever, get_semantic_retriever


@dataclass
class KnowledgeEntryData:
    """Data for creating a knowledge entry."""

    category: str
    title: str
    content: str
    source_video: Optional[str] = None
    source_timestamp: Optional[str] = None


class KnowledgeBase:
    """Manage knowledge base entries with embedding and retrieval."""

    def __init__(
        self,
        database_url: Optional[str] = None,
        embedding_generator: Optional[EmbeddingGenerator] = None,
        retriever: Optional[SemanticRetriever] = None,
        reranker: Optional[Reranker] = None,
    ):
        self.database_url = database_url or os.getenv("DATABASE_URL")
        self.embedding_generator = embedding_generator or get_embedding_generator()
        self.retriever = retriever or get_semantic_retriever()
        self.reranker = reranker or get_reranker()
        self._connection: Optional[psycopg.Connection] = None

    async def _get_connection(self) -> psycopg.Connection:
        """Get or create database connection."""
        if self._connection is None or self._connection.closed:
            if not self.database_url:
                raise ValueError("DATABASE_URL is required")

            url = self.database_url.replace("+asyncpg", "").replace("+psycopg", "")
            self._connection = await psycopg.AsyncConnection.connect(url)
            await register_vector(self._connection)

        return self._connection

    async def add_entry(self, entry: KnowledgeEntryData) -> int:
        """
        Add a knowledge entry with auto-generated embedding.

        Args:
            entry: The knowledge entry data.

        Returns:
            The ID of the created entry.
        """
        # Generate embedding
        embedding = await self.embedding_generator.generate(entry.content)

        conn = await self._get_connection()

        sql = """
            INSERT INTO knowledge_entries
                (category, title, content, embedding, source_video, source_timestamp)
            VALUES (%s, %s, %s, %s::vector, %s, %s)
            RETURNING id
        """

        cur = conn.cursor()
        try:
            await cur.execute(
                sql,
                (
                    entry.category,
                    entry.title,
                    entry.content,
                    embedding,
                    entry.source_video,
                    entry.source_timestamp,
                ),
            )
            result = await cur.fetchone()
            await conn.commit()
        finally:
            await cur.close()

        return result[0]

    async def add_entries_batch(self, entries: list[KnowledgeEntryData]) -> list[int]:
        """
        Add multiple knowledge entries with auto-generated embeddings.

        Args:
            entries: List of knowledge entry data.

        Returns:
            List of created entry IDs.
        """
        if not entries:
            return []

        # Generate embeddings in batch
        texts = [entry.content for entry in entries]
        embeddings = await self.embedding_generator.generate_batch(texts)

        conn = await self._get_connection()

        sql = """
            INSERT INTO knowledge_entries
                (category, title, content, embedding, source_video, source_timestamp)
            VALUES (%s, %s, %s, %s::vector, %s, %s)
            RETURNING id
        """

        ids = []
        cur = conn.cursor()
        try:
            for entry, embedding in zip(entries, embeddings):
                await cur.execute(
                    sql,
                    (
                        entry.category,
                        entry.title,
                        entry.content,
                        embedding,
                        entry.source_video,
                        entry.source_timestamp,
                    ),
                )
                result = await cur.fetchone()
                ids.append(result[0])

            await conn.commit()
        finally:
            await cur.close()

        return ids

    async def search(
        self,
        query: str,
        top_k: int = 10,
        top_n: int = 3,
        category: Optional[str] = None,
    ) -> list[RetrievalResult]:
        """
        End-to-end semantic search: retrieve then rerank.

        Args:
            query: The search query.
            top_k: Number of candidates to retrieve.
            top_n: Number of final results after reranking.
            category: Optional category filter.

        Returns:
            List of reranked results.
        """
        # Retrieve candidates
        candidates = await self.retriever.search(
            query=query,
            top_k=top_k,
            category=category,
        )

        # Rerank
        reranked = await self.reranker.rerank(
            query=query,
            candidates=candidates,
            top_n=top_n,
        )

        return reranked

    async def get_entry(self, entry_id: int) -> Optional[RetrievalResult]:
        """
        Get a knowledge entry by ID.

        Args:
            entry_id: The entry ID.

        Returns:
            The entry if found, None otherwise.
        """
        conn = await self._get_connection()

        sql = """
            SELECT id, category, title, content, source_video, source_timestamp
            FROM knowledge_entries
            WHERE id = %s
        """

        cur = conn.cursor()
        try:
            await cur.execute(sql, (entry_id,))
            row = await cur.fetchone()
        finally:
            await cur.close()

        if row is None:
            return None

        return RetrievalResult(
            id=row[0],
            category=row[1],
            title=row[2],
            content=row[3],
            similarity=1.0,  # Not applicable for direct lookup
            source_video=row[4],
            source_timestamp=row[5],
        )

    async def delete_entry(self, entry_id: int) -> bool:
        """
        Delete a knowledge entry.

        Args:
            entry_id: The entry ID.

        Returns:
            True if deleted, False if not found.
        """
        conn = await self._get_connection()

        sql = "DELETE FROM knowledge_entries WHERE id = %s"

        cur = conn.cursor()
        try:
            await cur.execute(sql, (entry_id,))
            await conn.commit()
            return cur.rowcount > 0
        finally:
            await cur.close()

    async def count(self) -> int:
        """Get total number of knowledge entries."""
        conn = await self._get_connection()

        sql = "SELECT COUNT(*) FROM knowledge_entries"

        cur = conn.cursor()
        try:
            await cur.execute(sql)
            result = await cur.fetchone()
            return result[0]
        finally:
            await cur.close()

    async def close(self):
        """Close database connection."""
        if self._connection and not self._connection.closed:
            await self._connection.close()


# Singleton instance
_default_knowledge_base: Optional[KnowledgeBase] = None


def get_knowledge_base() -> KnowledgeBase:
    """Get or create the default knowledge base."""
    global _default_knowledge_base
    if _default_knowledge_base is None:
        _default_knowledge_base = KnowledgeBase()
    return _default_knowledge_base
