"""Published-visibility gating, lexical anchoring, and ranking for knowledge search."""

from __future__ import annotations

import re
from typing import Any, LiteralString, cast

from openai import OpenAIError
from psycopg import InterfaceError, OperationalError
from psycopg_pool import PoolTimeout

from .knowledge_library_base import (
    ClipResult,
    KnowledgeLibraryBase,
    KnowledgeLibraryUnavailableError,
    SearchResult,
)

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


# Governed text sources that publish under the lexical hashing embedding. Under
# hashing a cosine score is not calibrated, so these sources require an
# independent lexical anchor before a published citation is returned.
GOVERNED_TEXT_SOURCE_TYPES = frozenset({"thought_forest_note", "medlineplus_topic"})

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


class KnowledgeSearchMixin(KnowledgeLibraryBase):
    """Read paths: embedding search, published gating, and intent reranking."""

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
        try:
            embedding = await self.embedding_generator.generate(query)
        except (OpenAIError, TimeoutError, ConnectionError, OSError) as exc:
            raise KnowledgeLibraryUnavailableError(
                "Knowledge embedding search dependency is unavailable"
            ) from exc
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
        try:
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
        except (OperationalError, InterfaceError, PoolTimeout, TimeoutError, OSError) as exc:
            raise KnowledgeLibraryUnavailableError(
                "Knowledge search database is unavailable"
            ) from exc

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
                    if result.source_type in GOVERNED_TEXT_SOURCE_TYPES
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
        score. Once the deny-first lexical gate admits a governed text candidate,
        the section-specific heading, reviewed claim body, and claim kind decide
        which nearby unit is the best match. This only orders candidates; it never
        makes an otherwise irrelevant result eligible.
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
        """Deny hash-only governed-text matches without a lexical topic anchor.

        Hashing embeddings are deterministic development/search embeddings, not a
        calibrated semantic model. A high cosine score can be a hash collision, so
        published governed text sources require an independent lexical anchor.
        Other source types and semantic embedding providers keep their existing
        behavior.
        """
        if embedding_provider != "hashing" or result.source_type not in GOVERNED_TEXT_SOURCE_TYPES:
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
