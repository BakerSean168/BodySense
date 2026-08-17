"""LLM-powered knowledge splitting — semantic analysis over transcript segments.

Uses an LLM to classify, segment, and title knowledge units instead of
keyword heuristics. Falls back to HeuristicSplitter on failure.
"""

from __future__ import annotations

import json
import logging
import re
from typing import Any

from ..ai import AiRequest, AIService
from ..ai.types import ChatMessage
from ..prompts.splitter import SPLITTER_SYSTEM_PROMPT, get_splitter_prompt
from .knowledge_pack import KnowledgeUnitCandidate, TranscriptSegment, slugify
from .splitter import HeuristicSplitter

logger = logging.getLogger(__name__)

# Max characters per batch to stay within LLM context limits
_MAX_BATCH_CHARS = 4000
# Overlap segments between batches for context continuity
_BATCH_OVERLAP = 2

_TYPE_SUFFIX_MAP = {
    "self_check": "自测与判断",
    "exercise": "改善动作",
    "warning": "风险提醒",
    "cause": "影响与成因",
    "explanation": "原理说明",
}


class LLMSplitter:
    """LLM-powered semantic knowledge splitting.

    Sends transcript text to an LLM which identifies topic boundaries,
    classifies each unit semantically, and generates titles/summaries.
    Falls back to HeuristicSplitter if LLM call fails.
    """

    async def split(
        self,
        transcript_segments: list[TranscriptSegment],
        problem_slug: str,
        problem_display_name: str,
    ) -> list[KnowledgeUnitCandidate]:
        if not transcript_segments:
            return []

        try:
            return await self._split_with_llm(
                transcript_segments, problem_slug, problem_display_name
            )
        except Exception:
            logger.warning("LLM splitting failed, falling back to heuristic", exc_info=True)
            fallback = HeuristicSplitter()
            return await fallback.split(transcript_segments, problem_slug, problem_display_name)

    async def _split_with_llm(
        self,
        segments: list[TranscriptSegment],
        problem_slug: str,
        problem_display_name: str,
    ) -> list[KnowledgeUnitCandidate]:
        """Core LLM splitting logic with batch handling."""
        ai = AIService()

        # Split long transcripts into batches
        batches = _segment_batches(segments)

        all_units: list[KnowledgeUnitCandidate] = []
        unit_counter: dict[str, int] = {}  # type → count for sequential numbering

        for batch in batches:
            transcript_text = _format_transcript(batch)
            user_prompt = get_splitter_prompt(
                transcript_text=transcript_text,
                problem_slug=problem_slug,
                problem_display_name=problem_display_name,
            )

            messages = [
                ChatMessage(role="system", content=SPLITTER_SYSTEM_PROMPT),
                ChatMessage(role="user", content=user_prompt),
            ]

            response = await ai.generate(
                AiRequest(
                    use_case="llm.json",
                    messages=messages,
                    temperature=0.3,
                    max_tokens=2048,
                )
            )
            raw_units = _parse_llm_response(response.text)

            for raw_unit in raw_units:
                unit_type = raw_unit.get("unit_type", "explanation")
                if unit_type not in _TYPE_SUFFIX_MAP:
                    unit_type = "explanation"

                unit_counter[unit_type] = unit_counter.get(unit_type, 0) + 1
                sequence = unit_counter[unit_type]

                # Ensure title follows the expected format
                title = raw_unit.get("title", "")
                if not title or len(title) < 5:
                    title = _fallback_title(problem_display_name, unit_type, sequence, raw_unit)

                # Validate time range
                start_sec = float(raw_unit.get("start_sec", 0))
                end_sec = float(raw_unit.get("end_sec", 0))

                # Find overlapping segments for evidence
                evidence_indices = _find_evidence_indices(segments, start_sec, end_sec)
                excerpt = _build_excerpt(segments, evidence_indices)

                category = (
                    f"exercise.{problem_slug}"
                    if unit_type == "exercise"
                    else f"posture.{problem_slug}"
                )

                tags = raw_unit.get("tags", [])
                if not isinstance(tags, list):
                    tags = []
                # Ensure base tags
                for required in [problem_slug, unit_type]:
                    if required not in tags:
                        tags.append(required)

                all_units.append(
                    KnowledgeUnitCandidate(
                        unit_key=f"{slugify(problem_slug)}-{unit_type}-{sequence:02d}",
                        problem_slug=problem_slug,
                        problem_display_name=problem_display_name,
                        category=category,
                        unit_type=unit_type,
                        title=title,
                        summary=raw_unit.get("summary", "")[:90],
                        body_markdown=_build_body_markdown(raw_unit, segments, evidence_indices),
                        source_start_sec=start_sec,
                        source_end_sec=end_sec,
                        evidence_segment_indices=evidence_indices,
                        tags=sorted(set(tags)),
                        transcript_excerpt=excerpt[:240],
                    )
                )

        return all_units


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _segment_batches(segments: list[TranscriptSegment]) -> list[list[TranscriptSegment]]:
    """Split segments into batches that respect the character limit.

    Each batch overlaps with the previous one by _BATCH_OVERLAP segments
    to maintain context continuity across batch boundaries.
    """
    if not segments:
        return []

    batches: list[list[TranscriptSegment]] = []
    current_batch: list[TranscriptSegment] = []
    current_chars = 0

    for segment in segments:
        seg_chars = len(segment.text)

        if current_batch and current_chars + seg_chars > _MAX_BATCH_CHARS:
            batches.append(current_batch)
            # Overlap: keep last N segments for context
            overlap = current_batch[-_BATCH_OVERLAP:]
            current_batch = list(overlap)
            current_chars = sum(len(s.text) for s in current_batch)

        current_batch.append(segment)
        current_chars += seg_chars

    if current_batch:
        batches.append(current_batch)

    return batches


def _format_transcript(segments: list[TranscriptSegment]) -> str:
    """Format segments as timestamped text for the LLM."""
    lines = []
    for seg in segments:
        ts = f"[{seg.start_sec:.1f}s-{seg.end_sec:.1f}s]"
        lines.append(f"{ts} {seg.text}")
    return "\n".join(lines)


def _parse_llm_response(content: str | None) -> list[dict[str, Any]]:
    """Parse LLM JSON response with 3-stage fallback."""
    if not content:
        raise ValueError("LLM returned empty response")

    # Stage 1: Direct parse
    try:
        data = json.loads(content)
        if isinstance(data, dict) and "units" in data:
            return data["units"]
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
            if isinstance(data, dict) and "units" in data:
                return data["units"]
    except (json.JSONDecodeError, ValueError):
        pass

    # Stage 3: Brace extraction
    try:
        start = content.index("{")
        end = content.rindex("}") + 1
        data = json.loads(content[start:end])
        if isinstance(data, dict) and "units" in data:
            return data["units"]
    except (json.JSONDecodeError, ValueError):
        pass

    raise ValueError(f"Failed to parse LLM splitter response: {content[:200]}")


def _find_evidence_indices(
    segments: list[TranscriptSegment],
    start_sec: float,
    end_sec: float,
) -> list[int]:
    """Find segment indices overlapping the given time range."""
    indices = []
    for seg in segments:
        if seg.end_sec > start_sec and seg.start_sec < end_sec:
            indices.append(seg.segment_index)
    return indices


def _build_excerpt(segments: list[TranscriptSegment], indices: list[int]) -> str:
    """Build a text excerpt from the given segment indices."""
    by_index = {s.segment_index: s.text for s in segments}
    parts = [by_index[i] for i in indices if i in by_index]
    return " ".join(parts)


def _build_body_markdown(
    raw_unit: dict[str, Any],
    segments: list[TranscriptSegment],
    evidence_indices: list[int],
) -> str:
    """Build body markdown from LLM output or fallback to auto-generated."""
    body = raw_unit.get("body_markdown", "")
    if body and len(body) > 20:
        return body

    # Fallback: auto-generate from transcript
    by_index = {s.segment_index: s for s in segments}
    lines = ["## 自动转录摘录"]
    for idx in evidence_indices:
        if idx in by_index:
            seg = by_index[idx]
            lines.append(f"- [{seg.timestamp}] {seg.text}")

    unit_type = raw_unit.get("unit_type", "explanation")
    if unit_type == "warning":
        lines.append("\n## 备注\n- 该片段包含风险或错误动作提醒，回答时应保守引用。")

    return "\n".join(lines)


def _fallback_title(
    problem_display_name: str,
    unit_type: str,
    sequence: int,
    raw_unit: dict[str, Any],
) -> str:
    """Generate a fallback title when LLM output is insufficient."""
    suffix = _TYPE_SUFFIX_MAP.get(unit_type, "内容整理")
    summary = raw_unit.get("summary", "")
    focus = re.sub(r"[，。！？!?\s]+", "", summary)[:14]
    if not focus:
        focus = "待补充"
    return f"{problem_display_name}{suffix} {sequence}: {focus}"
