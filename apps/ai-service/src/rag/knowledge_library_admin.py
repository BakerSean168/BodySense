"""Corpus visibility counts and inventory reads for the knowledge library."""

from __future__ import annotations

from typing import Any

from psycopg import sql

from .knowledge_library_base import KnowledgeLibraryBase, KnowledgeLibraryUnavailableError


class KnowledgeAdminMixin(KnowledgeLibraryBase):
    """Read-only corpus state: published counts and source inventory."""

    async def published_corpus_count(self, min_quality_score: float = 0.0) -> int:
        """Count online-visible published units without invoking embeddings.

        This lets evidence acquisition distinguish an actually empty published
        corpus from a non-empty corpus where one targeted query has no relevant
        result.
        """
        pool = self._require_pool()
        try:
            async with pool.connection() as conn:
                async with conn.cursor() as cur:
                    await cur.execute(
                        """
                        SELECT COUNT(*)
                        FROM knowledge_units ku
                        WHERE ku.lifecycle_status = 'published'
                          AND ku.publication_id IS NOT NULL
                          AND ku.published_version IS NOT NULL
                          AND ku.review_status IN ('reviewed', 'approved', 'curated')
                          AND COALESCE(ku.quality_score, 0.0) >= %s
                        """,
                        (min_quality_score,),
                    )
                    row = await cur.fetchone()
        except KnowledgeLibraryUnavailableError:
            raise
        except Exception as exc:
            raise KnowledgeLibraryUnavailableError(
                "published Knowledge corpus state is unavailable"
            ) from exc
        return int(row[0]) if row is not None else 0

    async def list_sources(self) -> list[dict[str, Any]]:
        pool = self._require_pool()
        async with pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(
                    """
                    SELECT id, source_key, title, author, problem_slug, ingest_status, created_at
                    FROM knowledge_sources
                    ORDER BY id DESC
                    """
                )
                rows = await cur.fetchall()
        return [
            {
                "id": row[0],
                "source_key": row[1],
                "title": row[2],
                "author": row[3],
                "problem_slug": row[4],
                "ingest_status": row[5],
                "created_at": row[6].isoformat(),
            }
            for row in rows
        ]

    _ALLOWED_TABLES = frozenset(
        {"knowledge_sources", "knowledge_segments", "knowledge_units", "knowledge_clips"}
    )

    async def stats(self) -> dict[str, int]:
        pool = self._require_pool()
        counts: dict[str, int] = {}
        async with pool.connection() as conn:
            async with conn.cursor() as cur:
                for table_name in self._ALLOWED_TABLES:
                    await cur.execute(
                        sql.SQL("SELECT COUNT(*) FROM {}").format(sql.Identifier(table_name))
                    )
                    row = await cur.fetchone()
                    counts[table_name] = row[0] if row is not None else 0
        return counts
