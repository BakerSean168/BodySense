"""Persistent knowledge library built around video sources, units, and clips."""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass, field
from typing import Any, Optional

import psycopg
from pgvector.psycopg import register_vector
from psycopg.types.json import Jsonb

from .embedding import EmbeddingGenerator, get_embedding_generator
from .knowledge_pack import GeneratedKnowledgePack, format_timestamp_range

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class ClipResult:
    """A clip associated with a search result."""

    id: int
    clip_key: str
    clip_type: str
    title: str
    file_path: str
    start_sec: float
    end_sec: float

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.start_sec, self.end_sec) or "00:00"


@dataclass(frozen=True)
class SearchResult:
    """Search result returned to the API layer."""

    id: int
    problem_slug: str
    category: str
    unit_type: str
    title: str
    summary: str
    body_markdown: str
    similarity: float
    source_title: str
    source_author: str
    source_start_sec: float
    source_end_sec: float
    tags: list[str] = field(default_factory=list)
    clips: list[ClipResult] = field(default_factory=list)

    @property
    def source_timestamp(self) -> str:
        return format_timestamp_range(self.source_start_sec, self.source_end_sec) or "00:00"


INTENT_KEYWORDS = {
    "definition": ["什么是", "定义", "是什么意思"],
    "self_check": ["自测", "判断", "筛查", "怎么测", "怎么看"],
    "exercise": [
        "练什么",
        "动作",
        "训练",
        "拉伸",
        "放松",
        "强化",
        "改善",
        "矫正",
        "纠正",
        "怎么练",
    ],
    "cause": ["原因", "为什么", "影响", "导致"],
    "warning": ["风险", "疼", "不适", "错误", "注意", "禁忌", "红旗", "就医", "不要自己练"],
    "muscle_imbalance": ["肌肉", "肌群", "紧张", "薄弱", "失衡"],
    "impact": ["影响", "表现", "风险", "麻木", "不适"],
    "habit": ["日常", "习惯", "工位", "办公", "学生党", "坐姿", "低头", "手机"],
}


class KnowledgeLibrary:
    """Persist and search structured knowledge units derived from videos."""

    def __init__(
        self,
        database_url: Optional[str] = None,
        embedding_generator: Optional[EmbeddingGenerator] = None,
    ):
        self.database_url = database_url or self._build_database_url()
        self.embedding_generator = embedding_generator or get_embedding_generator()
        self._connection: Optional[psycopg.Connection] = None

    def _build_database_url(self) -> str:
        env_url = os.getenv("DATABASE_URL")
        if env_url:
            return env_url.replace("+asyncpg", "").replace("+psycopg", "")

        host = os.getenv("DB_HOST", "127.0.0.1")
        port = os.getenv("DB_PORT", "5432")
        name = os.getenv("DB_NAME", "bodysense")
        user = os.getenv("DB_USER", "bodysense")
        password = os.getenv("DB_PASSWORD")
        if password is None:
            password = "bodysense123"
            logger.warning(
                "DB_PASSWORD not set, using default password. "
                "Set DB_PASSWORD in your .env file."
            )
        return f"postgresql://{user}:{password}@{host}:{port}/{name}"

    def _get_connection(self) -> psycopg.Connection:
        if self._connection is None or self._connection.closed:
            self._connection = psycopg.connect(self.database_url)
            register_vector(self._connection)
        return self._connection

    async def ingest_generated_pack(
        self,
        pack: GeneratedKnowledgePack,
        overwrite_source: bool = False,
    ) -> dict[str, Any]:
        """Insert a generated pack into the normalized knowledge schema."""
        conn = self._get_connection()

        existing_source_id: int | None = None
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM knowledge_sources WHERE source_key = %s",
                (pack.source.source_key,),
            )
            row = cur.fetchone()
            if row is not None:
                existing_source_id = row[0]

        if existing_source_id is not None:
            if not overwrite_source:
                return {
                    "source_id": existing_source_id,
                    "source_key": pack.source.source_key,
                    "status": "already_exists",
                }
            with conn.cursor() as cur:
                cur.execute("DELETE FROM knowledge_sources WHERE id = %s", (existing_source_id,))
            conn.commit()

        embedding_inputs = [
            "\n".join([unit.title, unit.summary, unit.body_markdown])
            for unit in pack.units
        ]
        embeddings = await self.embedding_generator.generate_batch(embedding_inputs)

        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO knowledge_sources (
                    source_key, source_type, title, author, problem_slug,
                    problem_display_name, original_file_path, language,
                    duration_sec, transcript_provider, transcript_model,
                    transcript_file_path, ingest_status, metadata
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                RETURNING id
                """,
                (
                    pack.source.source_key,
                    pack.source.source_type,
                    pack.source.title,
                    pack.source.author,
                    pack.source.problem_slug,
                    pack.source.problem_display_name,
                    pack.source.original_file_path,
                    pack.source.language,
                    pack.source.duration_sec,
                    pack.source.transcript_provider,
                    pack.source.transcript_model,
                    pack.source.transcript_file_path,
                    "ingested",
                    Jsonb(pack.source.metadata),
                ),
            )
            source_id = cur.fetchone()[0]

            for segment in pack.transcript_segments:
                cur.execute(
                    """
                    INSERT INTO knowledge_segments (
                        source_id, segment_index, start_sec, end_sec,
                        transcript, normalized_transcript, confidence, metadata
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        source_id,
                        segment.segment_index,
                        segment.start_sec,
                        segment.end_sec,
                        segment.text,
                        segment.text,
                        segment.confidence,
                        Jsonb({}),
                    ),
                )

            unit_id_by_key: dict[str, int] = {}
            for unit, embedding in zip(pack.units, embeddings, strict=False):
                cur.execute(
                    """
                    INSERT INTO knowledge_units (
                        source_id, unit_key, problem_slug, category, unit_type,
                        title, summary, body_markdown, source_start_sec,
                        source_end_sec, evidence_segment_indices, tags,
                        transcript_excerpt, review_status, embedding, metadata
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::vector, %s)
                    RETURNING id
                    """,
                    (
                        source_id,
                        unit.unit_key,
                        unit.problem_slug,
                        unit.category,
                        unit.unit_type,
                        unit.title,
                        unit.summary,
                        unit.body_markdown,
                        unit.source_start_sec,
                        unit.source_end_sec,
                        unit.evidence_segment_indices,
                        unit.tags,
                        unit.transcript_excerpt,
                        unit.review_status,
                        embedding,
                        Jsonb({"problem_display_name": unit.problem_display_name}),
                    ),
                )
                unit_id_by_key[unit.unit_key] = cur.fetchone()[0]

            for clip in pack.clips:
                cur.execute(
                    """
                    INSERT INTO knowledge_clips (
                        source_id, source_unit_id, clip_key, clip_type, title,
                        file_path, start_sec, end_sec, transcript_excerpt, notes, metadata
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        source_id,
                        unit_id_by_key.get(clip.source_unit_key),
                        clip.clip_key,
                        clip.clip_type,
                        clip.title,
                        clip.file_path,
                        clip.start_sec,
                        clip.end_sec,
                        clip.transcript_excerpt,
                        clip.notes,
                        Jsonb({}),
                    ),
                )

        conn.commit()
        return {
            "source_id": source_id,
            "source_key": pack.source.source_key,
            "status": "ingested",
            "segments": len(pack.transcript_segments),
            "units": len(pack.units),
            "clips": len(pack.clips),
        }

    async def search(
        self,
        query: str,
        top_k: int = 5,
        problem_slug: str | None = None,
        unit_type: str | None = None,
        include_unpublished: bool = False,
        min_quality_score: float = 0.0,
    ) -> list[SearchResult]:
        """Search the normalized knowledge units by semantic similarity."""
        embedding = await self.embedding_generator.generate(query)
        conn = self._get_connection()

        sql = """
            SELECT
                ku.id,
                ku.problem_slug,
                ku.category,
                ku.unit_type,
                ku.title,
                ku.summary,
                ku.body_markdown,
                ku.source_start_sec,
                ku.source_end_sec,
                ku.tags,
                ks.title AS source_title,
                ks.author AS source_author,
                1 - (ku.embedding <=> %s::vector) AS similarity
            FROM knowledge_units ku
            JOIN knowledge_sources ks ON ks.id = ku.source_id
            WHERE ku.embedding IS NOT NULL
        """
        params: list[Any] = [embedding]

        if not include_unpublished:
            sql += self._published_visibility_filter()
            params.append(min_quality_score)

        if problem_slug:
            sql += " AND ku.problem_slug = %s"
            params.append(problem_slug)

        if unit_type:
            sql += " AND ku.unit_type = %s"
            params.append(unit_type)

        candidate_limit = min(max(top_k * 6, top_k), 50)
        sql += " ORDER BY ku.embedding <=> %s::vector LIMIT %s"
        params.extend([embedding, candidate_limit])

        with conn.cursor() as cur:
            cur.execute(sql, params)
            rows = cur.fetchall()

        if not rows:
            return []

        result_ids = [row[0] for row in rows]
        clips_by_unit: dict[int, list[ClipResult]] = {result_id: [] for result_id in result_ids}
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT id, source_unit_id, clip_key, clip_type, title,
                       file_path, start_sec, end_sec
                FROM knowledge_clips
                WHERE source_unit_id = ANY(%s)
                ORDER BY id ASC
                """,
                (result_ids,),
            )
            for clip_row in cur.fetchall():
                clips_by_unit[clip_row[1]].append(
                    ClipResult(
                        id=clip_row[0],
                        clip_key=clip_row[2],
                        clip_type=clip_row[3],
                        title=clip_row[4],
                        file_path=clip_row[5],
                        start_sec=float(clip_row[6]),
                        end_sec=float(clip_row[7]),
                    )
                )

        results = [
            SearchResult(
                id=row[0],
                problem_slug=row[1],
                category=row[2],
                unit_type=row[3],
                title=row[4],
                summary=row[5],
                body_markdown=row[6],
                source_start_sec=float(row[7]),
                source_end_sec=float(row[8]),
                tags=row[9] or [],
                source_title=row[10],
                source_author=row[11],
                similarity=float(row[12]),
                clips=clips_by_unit.get(row[0], []),
            )
            for row in rows
        ]
        reranked = sorted(
            results,
            key=lambda result: result.similarity + self._intent_boost(query, result),
            reverse=True,
        )
        return reranked[:top_k]

    @staticmethod
    def _published_visibility_filter() -> str:
        """SQL predicate for knowledge that is safe to surface in user-facing search."""
        return """
            AND (
                ku.lifecycle_status IN ('published', 'reviewed')
                OR ku.review_status IN ('reviewed', 'approved', 'curated')
            )
            AND COALESCE(ku.quality_score, 0.0) >= %s
        """

    async def list_sources(self) -> list[dict[str, Any]]:
        """List ingested sources."""
        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT id, source_key, title, author, problem_slug, ingest_status, created_at
                FROM knowledge_sources
                ORDER BY id DESC
                """
            )
            rows = cur.fetchall()
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

    _ALLOWED_TABLES = frozenset({
        "knowledge_sources",
        "knowledge_segments",
        "knowledge_units",
        "knowledge_clips",
    })

    async def stats(self) -> dict[str, int]:
        """Return normalized knowledge table counts."""
        conn = self._get_connection()
        counts: dict[str, int] = {}
        with conn.cursor() as cur:
            for table_name in self._ALLOWED_TABLES:
                cur.execute(f"SELECT COUNT(*) FROM {table_name}")
                counts[table_name] = cur.fetchone()[0]
        return counts

    async def close(self):
        if self._connection and not self._connection.closed:
            self._connection.close()

    def _intent_boost(self, query: str, result: SearchResult) -> float:
        normalized_query = query.strip().lower()
        haystack = " ".join(
            [
                result.title.lower(),
                result.summary.lower(),
                result.body_markdown.lower(),
                " ".join(tag.lower() for tag in result.tags),
            ]
        )
        boost = 0.0

        for unit_type, keywords in INTENT_KEYWORDS.items():
            if (
                any(keyword in normalized_query for keyword in keywords)
                and result.unit_type == unit_type
            ):
                boost += 0.35

        for keyword in set(sum(INTENT_KEYWORDS.values(), [])):
            if keyword in normalized_query and keyword in haystack:
                boost += 0.06

        if "头前移" in normalized_query and "头前移" in haystack:
            boost += 0.05

        if result.clips:
            boost += 0.03

        return boost


_default_knowledge_library: Optional[KnowledgeLibrary] = None


def get_knowledge_library() -> KnowledgeLibrary:
    """Get or create the default normalized knowledge library."""
    global _default_knowledge_library
    if _default_knowledge_library is None:
        _default_knowledge_library = KnowledgeLibrary()
    return _default_knowledge_library
