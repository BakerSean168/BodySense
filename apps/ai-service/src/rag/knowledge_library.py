"""Persistent knowledge library built around video sources, units, and clips."""

from __future__ import annotations

import asyncio
import logging
import os
import re
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from typing import Any, AsyncIterator, LiteralString, Optional, cast

from pgvector.psycopg import register_vector_async
from psycopg import AsyncConnection, sql
from psycopg.rows import TupleRow
from psycopg.types.json import Jsonb
from psycopg_pool import AsyncConnectionPool

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
    unit_key: str = ""
    source_key: str = ""
    source_type: str = "video"
    unit_metadata: dict[str, Any] = field(default_factory=dict)
    source_metadata: dict[str, Any] = field(default_factory=dict)
    lifecycle_status: str = "generated"
    review_status: str = "generated"
    quality_score: float = 0.0
    publication_id: str = ""
    published_version: int | None = None
    publication_key: str = ""
    publication_batch_key: str = ""

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


_LEXICAL_QUERY_NOISE = (
    "什么是",
    "是什么意思",
    "是不是",
    "能不能",
    "为什么",
    "怎么样",
    "怎么",
    "如何",
    "应该",
    "需要",
    "可以",
    "哪些",
    "什么",
)
_LEXICAL_GENERIC_ANCHORS = {
    "处理",
    "训练",
    "评估",
    "测量",
    "定义",
    "问题",
    "情况",
    "方法",
    "建议",
    "症状",
    "风险",
}


_PUBLISHED_HASHING_INTERPRETATION_MARKERS = (
    "同一",
    "一样",
    "等于",
    "不等于",
    "一定",
    "只看",
    "仅凭",
    "能判断",
    "代表",
    "没有明确",
    "是不是",
)


CLAIM_KIND_INTENT_KEYWORDS = {
    "definition": ["什么是", "定义", "是什么意思"],
    "measurement_guidance": ["评估", "测量", "怎么测", "检查", "鉴别", "自测", "筛查"],
    "interpretation_boundary": ["一定", "是不是", "能不能", "直接", "等于", "不等于", "误解"],
    "association": ["相关", "关系", "关联", "会导致", "风险"],
    "mechanism_hypothesis": ["原因", "为什么", "机制", "导致", "病因"],
    "safety_guidance": ["风险", "红旗", "就医", "升级", "紧急", "安全", "不要自己练"],
    "intervention_option": ["练什么", "训练", "拉伸", "放松", "改善", "怎么练", "治疗"],
    "dosage_guidance": ["训练量", "强度", "频率", "剂量", "次数", "组数", "进阶", "progression"],
    "outcome_reassessment": ["复测", "重新评估", "效果", "结果", "进展", "验证"],
}


class KnowledgeLibraryUnavailableError(RuntimeError):
    """Raised when the lifecycle-owned async knowledge pool is unavailable."""


KNOWLEDGE_CONNECT_TIMEOUT_SECONDS = 5.0
KNOWLEDGE_POOL_MIN_SIZE = 1
KNOWLEDGE_POOL_MAX_SIZE = 8


async def _configure_vector_connection(conn: AsyncConnection[TupleRow]) -> None:
    await register_vector_async(conn)


class KnowledgeLibrary:
    """Persist and search structured knowledge with lifecycle-owned async I/O."""

    def __init__(
        self,
        database_url: Optional[str] = None,
        embedding_generator: Optional[EmbeddingGenerator] = None,
        *,
        pool: AsyncConnectionPool[AsyncConnection[TupleRow]] | None = None,
        owns_pool: bool | None = None,
    ):
        self.database_url = database_url or self._build_database_url()
        self.embedding_generator = embedding_generator or get_embedding_generator()
        self._pool = pool
        self._owns_pool = (pool is None) if owns_pool is None else owns_pool
        self._init_lock = asyncio.Lock()

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
                "DB_PASSWORD not set, using default password. Set DB_PASSWORD in your .env file."
            )
        return f"postgresql://{user}:{password}@{host}:{port}/{name}"

    async def initialize(self) -> "KnowledgeLibrary":
        """Open the bounded async pool once; startup failures surface promptly."""
        if self._pool is not None:
            return self
        async with self._init_lock:
            if self._pool is not None:
                return self
            pool: AsyncConnectionPool[AsyncConnection[TupleRow]] = AsyncConnectionPool(
                self.database_url,
                min_size=int(os.getenv("KNOWLEDGE_POOL_MIN_SIZE", str(KNOWLEDGE_POOL_MIN_SIZE))),
                max_size=int(os.getenv("KNOWLEDGE_POOL_MAX_SIZE", str(KNOWLEDGE_POOL_MAX_SIZE))),
                open=False,
                configure=_configure_vector_connection,
            )
            try:
                await pool.open(wait=True, timeout=KNOWLEDGE_CONNECT_TIMEOUT_SECONDS)
            except Exception as exc:
                await pool.close()
                raise KnowledgeLibraryUnavailableError(
                    "knowledge Postgres pool unavailable during bounded startup"
                ) from exc
            self._pool = pool
            self._owns_pool = True
            logger.info("Initialized async KnowledgeLibrary Postgres pool")
        return self

    def _require_pool(self) -> AsyncConnectionPool[AsyncConnection[TupleRow]]:
        if self._pool is None:
            raise KnowledgeLibraryUnavailableError(
                "KnowledgeLibrary is not initialized; use the FastAPI lifespan "
                "or an explicit test pool"
            )
        return self._pool

    async def assert_sources_overwritable(self, source_keys: list[str]) -> None:
        """Fail before batch writes if any source is protected by publication state."""
        normalized = sorted({key for key in source_keys if key})
        if not normalized:
            return
        pool = self._require_pool()
        async with pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(
                    """
                    SELECT ks.source_key
                    FROM knowledge_sources ks
                    WHERE ks.source_key = ANY(%s)
                      AND EXISTS (
                          SELECT 1
                          FROM knowledge_units ku
                          WHERE ku.source_id = ks.id
                            AND (
                                ku.lifecycle_status = 'published'
                                OR ku.publication_id IS NOT NULL
                            )
                      )
                    ORDER BY ks.source_key
                    """,
                    (normalized,),
                )
                rows = await cur.fetchall()
        if rows:
            protected = ", ".join(str(row[0]) for row in rows)
            raise RuntimeError(
                "cannot overwrite knowledge batch; published/publication-linked sources: "
                f"{protected}"
            )

    async def ingest_generated_pack(
        self,
        pack: GeneratedKnowledgePack,
        overwrite_source: bool = False,
    ) -> dict[str, Any]:
        """Insert a generated pack atomically using an async transaction."""
        embedding_inputs = [
            "\n".join([unit.title, unit.summary, unit.body_markdown]) for unit in pack.units
        ]
        embeddings = await self.embedding_generator.generate_batch(embedding_inputs)
        if len(embeddings) != len(pack.units):
            raise RuntimeError("embedding count does not match generated knowledge unit count")
        embedding_identity = self.embedding_generator.identity()
        expected_dimension = int(embedding_identity["dimension"])
        if any(len(embedding) != expected_dimension for embedding in embeddings):
            raise RuntimeError("embedding dimension does not match embedding identity")
        pool = self._require_pool()

        async with pool.connection() as conn:
            async with conn.transaction():
                async with conn.cursor() as cur:
                    await cur.execute(
                        """
                        SELECT id, ingest_status, source_type, title, author, problem_slug,
                               problem_display_name, language, license_status, content_hash,
                               provenance, registered_by, registered_at
                        FROM knowledge_sources
                        WHERE source_key = %s
                        """,
                        (pack.source.source_key,),
                    )
                    row = await cur.fetchone()
                    if row is None:
                        raise RuntimeError("knowledge source must be registered before ingestion")

                    source_id = row[0]
                    ingest_status = row[1]
                    registered_identity = (row[2], row[3], row[4], row[5], row[6], row[7])
                    pack_identity = (
                        pack.source.source_type,
                        pack.source.title,
                        pack.source.author,
                        pack.source.problem_slug,
                        pack.source.problem_display_name,
                        pack.source.language,
                    )
                    if registered_identity != pack_identity:
                        raise RuntimeError(
                            "generated pack identity does not match registered knowledge source"
                        )
                    if (
                        row[8] not in {"verified_reuse", "citation_only", "owned", "public_domain"}
                        or not row[9]
                        or not row[10]
                        or not row[11]
                        or row[12] is None
                    ):
                        raise RuntimeError(
                            "registered knowledge source is missing "
                            "license/content/provenance authority"
                        )

                    if ingest_status != "registered" and not overwrite_source:
                        return {
                            "source_id": source_id,
                            "source_key": pack.source.source_key,
                            "status": "already_exists",
                        }
                    if ingest_status != "registered":
                        await cur.execute(
                            """
                            SELECT COUNT(*)
                            FROM knowledge_units
                            WHERE source_id = %s
                              AND (lifecycle_status = 'published' OR publication_id IS NOT NULL)
                            """,
                            (source_id,),
                        )
                        protected_row = await cur.fetchone()
                        protected_count = int(protected_row[0]) if protected_row is not None else 0
                        if protected_count > 0:
                            raise RuntimeError(
                                "cannot overwrite a knowledge source with published/"
                                "publication-linked units; rollback or create a new "
                                "immutable source version first"
                            )
                        await cur.execute(
                            "DELETE FROM knowledge_clips WHERE source_id = %s",
                            (source_id,),
                        )
                        await cur.execute(
                            "DELETE FROM knowledge_segments WHERE source_id = %s",
                            (source_id,),
                        )
                        await cur.execute(
                            "DELETE FROM knowledge_units WHERE source_id = %s",
                            (source_id,),
                        )

                    await cur.execute(
                        """
                        UPDATE knowledge_sources
                        SET duration_sec = %s,
                            transcript_provider = %s,
                            transcript_model = %s,
                            transcript_file_path = %s,
                            ingest_status = 'ingested',
                            metadata = COALESCE(metadata, '{}'::jsonb) || %s
                        WHERE id = %s
                        """,
                        (
                            pack.source.duration_sec,
                            pack.source.transcript_provider,
                            pack.source.transcript_model,
                            pack.source.transcript_file_path,
                            Jsonb(pack.source.metadata),
                            source_id,
                        ),
                    )

                    for segment in pack.transcript_segments:
                        await cur.execute(
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
                                Jsonb(segment.metadata),
                            ),
                        )

                    unit_id_by_key: dict[str, int] = {}
                    for unit, embedding in zip(pack.units, embeddings, strict=False):
                        await cur.execute(
                            """
                            INSERT INTO knowledge_units (
                                source_id, unit_key, problem_slug, category, unit_type,
                                title, summary, body_markdown, source_start_sec,
                                source_end_sec, evidence_segment_indices, tags,
                                transcript_excerpt, review_status, lifecycle_status,
                                quality_score, content_hash, embedding, metadata
                            )
                            VALUES (
                                %s, %s, %s, %s, %s, %s, %s, %s,
                                %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::vector, %s
                            )
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
                                unit.lifecycle_status,
                                unit.quality_score,
                                unit.content_hash,
                                embedding,
                                Jsonb(
                                    {
                                        **unit.metadata,
                                        "problem_display_name": unit.problem_display_name,
                                        "embedding_identity": embedding_identity,
                                    }
                                ),
                            ),
                        )
                        row = await cur.fetchone()
                        if row is None:
                            raise RuntimeError("Failed to persist knowledge unit row")
                        unit_id_by_key[unit.unit_key] = row[0]

                    for clip in pack.clips:
                        await cur.execute(
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
        source_type: str | None = None,
        include_unpublished: bool = False,
        min_quality_score: float = 0.0,
    ) -> list[SearchResult]:
        """Search normalized knowledge units without blocking the event loop."""
        embedding = await self.embedding_generator.generate(query)
        query_sql = """
            SELECT
                ku.id, ku.problem_slug, ku.category, ku.unit_type, ku.title,
                ku.summary, ku.body_markdown, ku.source_start_sec, ku.source_end_sec,
                ku.tags, ks.title AS source_title, ks.author AS source_author,
                ku.unit_key, ku.metadata, ks.source_key, ks.source_type, ks.metadata,
                ku.lifecycle_status, ku.review_status, COALESCE(ku.quality_score, 0.0),
                COALESCE(ku.publication_id::text, ''), ku.published_version,
                COALESCE(kp.publication_key, ''), COALESCE(kp.publication_batch_key, ''),
                1 - (ku.embedding <=> %s::vector) AS similarity
            FROM knowledge_units ku
            JOIN knowledge_sources ks ON ks.id = ku.source_id
            LEFT JOIN knowledge_publications kp ON kp.id = ku.publication_id
            WHERE ku.embedding IS NOT NULL
        """
        params: list[Any] = [embedding]
        if not include_unpublished:
            query_sql += self._published_visibility_filter()
            params.append(min_quality_score)
        if problem_slug:
            query_sql += " AND ku.problem_slug = %s"
            params.append(problem_slug)
        if unit_type:
            query_sql += " AND ku.unit_type = %s"
            params.append(unit_type)
        if source_type:
            query_sql += " AND ks.source_type = %s"
            params.append(source_type)
        candidate_limit = min(max(top_k * 6, top_k), 50)
        query_sql += " ORDER BY ku.embedding <=> %s::vector LIMIT %s"
        params.extend([embedding, candidate_limit])

        pool = self._require_pool()
        async with pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(cast("LiteralString", query_sql), params)
                rows = await cur.fetchall()
                if not rows:
                    return []

                result_ids = [row[0] for row in rows]
                clips_by_unit: dict[int, list[ClipResult]] = {
                    result_id: [] for result_id in result_ids
                }
                await cur.execute(
                    """
                    SELECT id, source_unit_id, clip_key, clip_type, title,
                           file_path, start_sec, end_sec
                    FROM knowledge_clips
                    WHERE source_unit_id = ANY(%s)
                    ORDER BY id ASC
                    """,
                    (result_ids,),
                )
                clip_rows = await cur.fetchall()

        for clip_row in clip_rows:
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
                unit_key=row[12] or "",
                unit_metadata=row[13] or {},
                source_key=row[14] or "",
                source_type=row[15] or "video",
                source_metadata=row[16] or {},
                lifecycle_status=row[17] or "generated",
                review_status=row[18] or "generated",
                quality_score=float(row[19] or 0.0),
                publication_id=row[20] or "",
                published_version=row[21],
                publication_key=row[22] or "",
                publication_batch_key=row[23] or "",
                similarity=float(row[24]),
                clips=clips_by_unit.get(row[0], []),
            )
            for row in rows
        ]
        if not include_unpublished:
            embedding_provider = str(getattr(self.embedding_generator, "provider", "") or "")
            results = [
                result
                for result in results
                if self._passes_published_relevance_gate(
                    query, result, embedding_provider=embedding_provider
                )
            ]
        if not include_unpublished and embedding_provider == "hashing":
            reranked = sorted(
                results,
                key=lambda result: (
                    self._published_hashing_rerank_score(query, result)
                    if result.source_type == "thought_forest_note"
                    else result.similarity + self._intent_boost(query, result)
                ),
                reverse=True,
            )
        else:
            reranked = sorted(
                results,
                key=lambda result: result.similarity + self._intent_boost(query, result),
                reverse=True,
            )
        return reranked[:top_k]

    @staticmethod
    def _meaningful_query_anchors(query: str) -> set[str]:
        normalized = query.strip().lower()
        for phrase in _LEXICAL_QUERY_NOISE:
            normalized = normalized.replace(phrase, "")
        anchors: set[str] = set()
        for token in re.findall(r"[a-z0-9][a-z0-9._+-]{2,}", normalized):
            anchors.add(token)
        for run in re.findall(r"[\u4e00-\u9fff]+", normalized):
            if len(run) == 1:
                continue
            max_n = min(4, len(run))
            for n in range(2, max_n + 1):
                for index in range(len(run) - n + 1):
                    gram = run[index : index + n]
                    if gram not in _LEXICAL_GENERIC_ANCHORS:
                        anchors.add(gram)
        return anchors

    @classmethod
    def _has_meaningful_lexical_anchor(cls, query: str, result: SearchResult) -> bool:
        anchors = cls._meaningful_query_anchors(query)
        if not anchors:
            return False
        primary_haystack = " ".join(
            [
                result.title.lower(),
                result.summary.lower(),
                " ".join(tag.lower() for tag in result.tags),
            ]
        )
        body_haystack = result.body_markdown.lower()
        return any(
            anchor in primary_haystack or (len(anchor) >= 3 and anchor in body_haystack)
            for anchor in anchors
        )

    @staticmethod
    def _lexical_units(value: str) -> set[str]:
        normalized = value.strip().lower()
        units = {
            token
            for token in re.findall(r"[a-z0-9][a-z0-9._+-]{2,}", normalized)
            if token not in _LEXICAL_GENERIC_ANCHORS
        }
        for run in re.findall(r"[\u4e00-\u9fff]+", normalized):
            if len(run) == 1:
                continue
            if len(run) == 2:
                units.add(run)
                continue
            for index in range(len(run) - 1):
                gram = run[index : index + 2]
                if gram not in _LEXICAL_GENERIC_ANCHORS:
                    units.add(gram)
        return units

    @classmethod
    def _lexical_coverage(cls, query: str, text: str) -> float:
        query_units = cls._lexical_units(query)
        if not query_units:
            return 0.0
        text_units = cls._lexical_units(text)
        return len(query_units & text_units) / len(query_units)

    @staticmethod
    def _section_heading(result: SearchResult) -> str:
        locator = dict(result.unit_metadata.get("source_locator") or {})
        heading_path = locator.get("heading_path") or []
        if isinstance(heading_path, list) and heading_path:
            return str(heading_path[-1])
        if " · " in result.title:
            return result.title.rsplit(" · ", 1)[-1]
        return result.title

    @staticmethod
    def _published_hashing_claim_intent_bonus(query: str, result: SearchResult) -> float:
        normalized = query.strip().lower()
        claim = dict(result.unit_metadata.get("claim_candidate") or {})
        claim_kind = str(claim.get("claim_kind") or "")
        if claim_kind == "definition" and any(
            marker in normalized for marker in ("什么是", "定义", "是什么意思")
        ):
            return 0.18
        if claim_kind == "interpretation_boundary" and any(
            marker in normalized for marker in _PUBLISHED_HASHING_INTERPRETATION_MARKERS
        ):
            return 0.10
        return 0.0

    def _published_hashing_rerank_score(self, query: str, result: SearchResult) -> float:
        """Rerank admitted hashing candidates using section-local lexical evidence.

        The hashing embedding is intentionally not treated as a calibrated semantic
        score. Once the deny-first lexical gate admits a published Thought Forest
        candidate, the section-specific heading, reviewed claim body, and claim kind
        decide which nearby unit is the best match. This only orders candidates; it
        never makes an otherwise irrelevant result eligible.
        """
        heading = self._section_heading(result)
        heading_coverage = self._lexical_coverage(query, heading)
        body_coverage = self._lexical_coverage(
            query,
            " ".join([result.summary, result.body_markdown]),
        )
        return (
            result.similarity
            + self._intent_boost(query, result)
            + self._published_hashing_claim_intent_bonus(query, result)
            + (0.45 * heading_coverage)
            + (0.40 * body_coverage)
        )

    @classmethod
    def _passes_published_relevance_gate(
        cls,
        query: str,
        result: SearchResult,
        *,
        embedding_provider: str,
    ) -> bool:
        """Deny hash-only Thought Forest matches without a lexical topic anchor.

        Hashing embeddings are deterministic development/search embeddings, not a
        calibrated semantic model. A high cosine score can be a hash collision,
        so published Thought Forest citations require an independent lexical anchor.
        Other source types and semantic embedding providers keep their existing behavior.
        """
        if embedding_provider != "hashing" or result.source_type != "thought_forest_note":
            return True
        return cls._has_meaningful_lexical_anchor(query, result)

    @staticmethod
    def _published_visibility_filter() -> str:
        """SQL predicate for knowledge safe to surface in user-facing search."""
        return """
            AND ku.lifecycle_status = 'published'
            AND ku.publication_id IS NOT NULL
            AND ku.published_version IS NOT NULL
            AND ku.review_status IN ('reviewed', 'approved', 'curated')
            AND COALESCE(ku.quality_score, 0.0) >= %s
        """

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

    async def close(self) -> None:
        pool = self._pool
        self._pool = None
        if pool is not None and self._owns_pool:
            await pool.close()

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
        claim_candidate = dict(result.unit_metadata.get("claim_candidate") or {})
        claim_kind = str(claim_candidate.get("claim_kind") or "")
        claim_keywords = CLAIM_KIND_INTENT_KEYWORDS.get(claim_kind, [])
        matched_claim_keywords = [
            keyword for keyword in claim_keywords if keyword in normalized_query
        ]
        if matched_claim_keywords and any(
            keyword in haystack for keyword in matched_claim_keywords
        ):
            boost += 0.10
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
    """Return the lifecycle-owned instance; this function never performs I/O."""
    global _default_knowledge_library
    if _default_knowledge_library is None:
        _default_knowledge_library = KnowledgeLibrary()
    return _default_knowledge_library


async def initialize_knowledge_library() -> KnowledgeLibrary:
    library = get_knowledge_library()
    await library.initialize()
    return library


async def shutdown_knowledge_library() -> None:
    global _default_knowledge_library
    library = _default_knowledge_library
    _default_knowledge_library = None
    if library is not None:
        await library.close()


@asynccontextmanager
async def knowledge_library_lifespan() -> AsyncIterator[None]:
    await initialize_knowledge_library()
    try:
        yield
    finally:
        await shutdown_knowledge_library()
