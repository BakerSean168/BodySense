"""AI-assisted knowledge unit refinement.

Uses an LLM to polish auto-generated knowledge units: improving titles,
summaries, body markdown, and adding quality scores. Each unit is processed
independently so failures don't block the entire pack.
"""

from __future__ import annotations

import asyncio
import json
import logging
from typing import Any

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage
from ..prompts.curator import CURATOR_SYSTEM_PROMPT, get_curator_prompt
from .knowledge_pack import (
    GeneratedKnowledgePack,
    KnowledgeUnitCandidate,
)

logger = logging.getLogger(__name__)

# Max concurrent LLM calls to avoid API rate limits
_MAX_CONCURRENCY = 3

# Units scoring below this threshold get flagged for human review
_QUALITY_THRESHOLD = 0.6


class AICurator:
    """LLM-assisted knowledge unit refinement.

    Processes each unit independently with concurrency control.
    Failed units are kept in their original form (non-blocking).
    Low-quality units get flagged with review_status="ai_flagged".
    """

    def __init__(self, max_concurrency: int = _MAX_CONCURRENCY):
        self._semaphore = asyncio.Semaphore(max_concurrency)

    async def refine_pack(
        self,
        pack: GeneratedKnowledgePack,
    ) -> GeneratedKnowledgePack:
        """Refine all units in a knowledge pack.

        Returns a new pack with refined units. Original pack is not modified.
        """
        if not pack.units:
            return pack

        refined_units = await asyncio.gather(
            *[self._refine_unit_safe(unit, pack.source.problem_display_name) for unit in pack.units]
        )

        return GeneratedKnowledgePack(
            source=pack.source,
            artifact_dir=pack.artifact_dir,
            transcript_segments=pack.transcript_segments,
            units=list(refined_units),
            clips=pack.clips,
        )

    async def _refine_unit_safe(
        self,
        unit: KnowledgeUnitCandidate,
        problem_display_name: str,
    ) -> KnowledgeUnitCandidate:
        """Refine a single unit with error handling (never raises)."""
        try:
            async with self._semaphore:
                return await self._refine_unit(unit, problem_display_name)
        except Exception:
            logger.warning(
                "AI refinement failed for unit %s, keeping original",
                unit.unit_key,
                exc_info=True,
            )
            return unit

    async def _refine_unit(
        self,
        unit: KnowledgeUnitCandidate,
        problem_display_name: str,
    ) -> KnowledgeUnitCandidate:
        """Refine a single knowledge unit via LLM."""
        ai = AIService()

        user_prompt = get_curator_prompt(
            title=unit.title,
            summary=unit.summary,
            body_markdown=unit.body_markdown,
            tags=unit.tags,
            unit_type=unit.unit_type,
            problem_display_name=problem_display_name,
            timestamp=unit.source_timestamp,
        )

        messages = [
            ChatMessage(role="system", content=CURATOR_SYSTEM_PROMPT),
            ChatMessage(role="user", content=user_prompt),
        ]

        response = await ai.generate(AiRequest(
            use_case="llm.json",
            messages=messages,
            temperature=0.3,
            max_tokens=2048,
        ))
        result = _parse_curator_response(response.text)

        # Determine review status based on quality score
        quality_score = float(result.get("quality_score", 0.5))
        review_status = unit.review_status
        if quality_score < _QUALITY_THRESHOLD:
            review_status = "ai_flagged"

        # Merge LLM output with original unit, preserving structural fields
        return KnowledgeUnitCandidate(
            unit_key=unit.unit_key,
            problem_slug=unit.problem_slug,
            problem_display_name=unit.problem_display_name,
            category=unit.category,
            unit_type=unit.unit_type,
            title=result.get("title", unit.title),
            summary=result.get("summary", unit.summary),
            body_markdown=result.get("body_markdown", unit.body_markdown),
            source_start_sec=unit.source_start_sec,
            source_end_sec=unit.source_end_sec,
            evidence_segment_indices=unit.evidence_segment_indices,
            tags=sorted(set(result.get("tags", unit.tags))),
            transcript_excerpt=unit.transcript_excerpt,
            review_status=review_status,
        )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _parse_curator_response(content: str | None) -> dict[str, Any]:
    """Parse LLM curator response with 3-stage fallback."""
    if not content:
        raise ValueError("LLM curator returned empty response")

    # Stage 1: Direct parse
    try:
        data = json.loads(content)
        if isinstance(data, dict):
            return data
    except json.JSONDecodeError:
        pass

    # Stage 2: Markdown code block extraction
    try:
        parts = content.split("```")
        for part in parts:
            cleaned = part.strip()
            if cleaned.startswith("json"):
                cleaned = cleaned[4:].strip()
            data = json.loads(cleaned)
            if isinstance(data, dict):
                return data
    except (json.JSONDecodeError, ValueError):
        pass

    # Stage 3: Brace extraction
    try:
        start = content.index("{")
        end = content.rindex("}") + 1
        data = json.loads(content[start:end])
        if isinstance(data, dict):
            return data
    except (json.JSONDecodeError, ValueError):
        pass

    raise ValueError(f"Failed to parse LLM curator response: {content[:200]}")
