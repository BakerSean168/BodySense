"""Knowledge splitting strategies — convert transcript segments to knowledge units.

Provides a ``Splitter`` protocol with two implementations:

- ``HeuristicSplitter`` — zero-cost keyword rules + sliding window (default)
- ``LLMSplitter`` — LLM-powered semantic splitting (in ``ai_splitter.py``)

Select via ``SPLITTER_PROVIDER`` env var: ``heuristic`` | ``llm``.
"""

from __future__ import annotations

import os
import re
from collections import Counter
from typing import Protocol

from .knowledge_pack import (
    KnowledgeUnitCandidate,
    TranscriptSegment,
    slugify,
)

# Only these unit types export video clips
CLIP_WORTHY_TYPES: set[str] = {"self_check", "exercise", "warning"}


# ---------------------------------------------------------------------------
# Protocol
# ---------------------------------------------------------------------------


class Splitter(Protocol):
    """Strategy interface for knowledge splitting."""

    async def split(
        self,
        transcript_segments: list[TranscriptSegment],
        problem_slug: str,
        problem_display_name: str,
    ) -> list[KnowledgeUnitCandidate]:
        """Split transcript segments into knowledge units."""
        ...


# ---------------------------------------------------------------------------
# Factory
# ---------------------------------------------------------------------------


def get_splitter(provider: str | None = None, configuration_id: str | None = None) -> Splitter:
    """Create a splitter based on configuration.

    Priority: explicit ``provider`` argument > ``SPLITTER_PROVIDER`` env var > ``heuristic``.
    """
    name = (provider or os.getenv("SPLITTER_PROVIDER", "heuristic")).strip().lower()

    if name == "heuristic":
        return HeuristicSplitter()

    if name == "llm":
        from .ai_splitter import LLMSplitter

        return LLMSplitter(configuration_id=configuration_id)

    raise ValueError(f"Unsupported splitter provider '{name}'. Supported: heuristic, llm")


# ---------------------------------------------------------------------------
# Heuristic splitter (default)
# ---------------------------------------------------------------------------


class HeuristicSplitter:
    """Keyword-based classification + sliding-window grouping.

    Zero cost, zero external dependencies, can run offline.
    Output is a "usable draft"; human or AI curation is the quality gate.
    """

    async def split(
        self,
        transcript_segments: list[TranscriptSegment],
        problem_slug: str,
        problem_display_name: str,
    ) -> list[KnowledgeUnitCandidate]:
        if not transcript_segments:
            return []

        # --- Step 1: Sliding window grouping ---
        grouped_segments: list[list[TranscriptSegment]] = []
        current_group: list[TranscriptSegment] = []
        current_types: list[str] = []

        for segment in transcript_segments:
            segment_type = classify_text(segment.text)

            if current_group and _should_split_group(
                current_group, current_types, segment, segment_type
            ):
                grouped_segments.append(current_group)
                current_group = []
                current_types = []

            current_group.append(segment)
            current_types.append(segment_type)

        if current_group:
            grouped_segments.append(current_group)

        # --- Step 2: Convert groups to knowledge unit candidates ---
        return _groups_to_units(grouped_segments, problem_slug, problem_display_name)


# ---------------------------------------------------------------------------
# Convenience function (backward compatible)
# ---------------------------------------------------------------------------


async def build_knowledge_units(
    transcript_segments: list[TranscriptSegment],
    problem_slug: str,
    problem_display_name: str,
) -> list[KnowledgeUnitCandidate]:
    """Split transcript segments into knowledge units using the default splitter."""
    splitter = get_splitter()
    return await splitter.split(transcript_segments, problem_slug, problem_display_name)


# ---------------------------------------------------------------------------
# Shared helpers (used by both HeuristicSplitter and LLMSplitter)
# ---------------------------------------------------------------------------


def classify_text(text: str) -> str:
    """Classify a transcript segment into one of 5 types using keyword rules.

    Classification priority (highest to lowest):
        self_check → exercise → warning → cause → explanation (fallback)

    Type meanings:
        - self_check:  Self-assessment methods ("从侧面看", "耳垂", "肩峰")
        - exercise:    Improvement exercises ("拉伸", "第一步", "毛巾")
        - warning:     Risk warnings ("不要", "疼", "代偿")
        - cause:       Cause analysis ("因为", "导致", "习惯")
        - explanation: Fallback type for general explanations
    """
    rules = [
        (
            "self_check",
            [
                "自测",
                "测试",
                "筛查",
                "摔查",
                "判断",
                "从侧面看",
                "侧面看",
                "自然放松",
                "站姿",
                "耳垂",
                "肩峰",
                "前方",
                "观察",
            ],
        ),
        (
            "exercise",
            [
                "动作",
                "训练",
                "练习",
                "拉伸",
                "放松",
                "激活",
                "回收",
                "后缩",
                "伸展",
                "处理方法",
                "第一步",
                "第二步",
                "第三步",
                "准备一个",
                "保留30秒",
                "两组",
                "网球",
                "毛巾",
                "旋转",
                "强化",
                "关节松动",
            ],
        ),
        (
            "warning",
            ["不要", "避免", "隐患", "疼", "酸痛", "不适", "错误", "代偿", "严重"],
        ),
        (
            "cause",
            ["因为", "导致", "造成", "影响", "问题", "长期", "习惯"],
        ),
    ]

    for unit_type, keywords in rules:
        if any(keyword in text for keyword in keywords):
            return unit_type

    return "explanation"


def _groups_to_units(
    grouped_segments: list[list[TranscriptSegment]],
    problem_slug: str,
    problem_display_name: str,
) -> list[KnowledgeUnitCandidate]:
    """Convert grouped segments into KnowledgeUnitCandidate list."""
    units: list[KnowledgeUnitCandidate] = []
    type_counters: Counter[str] = Counter()

    for index, group in enumerate(grouped_segments, start=1):
        texts = [segment.text for segment in group]
        combined_text = " ".join(texts)

        dominant_type = _dominant_type(texts)
        type_counters[dominant_type] += 1

        summary = _make_summary(combined_text)
        title = _make_title(
            problem_display_name=problem_display_name,
            unit_type=dominant_type,
            sequence=type_counters[dominant_type],
            combined_text=combined_text,
        )

        category = (
            f"exercise.{problem_slug}" if dominant_type == "exercise" else f"posture.{problem_slug}"
        )

        body_lines = [
            "## 自动转录摘录",
            *[f"- [{segment.timestamp}] {segment.text}" for segment in group],
        ]

        if dominant_type == "warning":
            body_lines.append("\n## 备注\n- 该片段包含风险或错误动作提醒，回答时应保守引用。")

        units.append(
            KnowledgeUnitCandidate(
                unit_key=f"{slugify(problem_slug)}-{dominant_type}-{index:02d}",
                problem_slug=problem_slug,
                problem_display_name=problem_display_name,
                category=category,
                unit_type=dominant_type,
                title=title,
                summary=summary,
                body_markdown="\n".join(body_lines),
                source_start_sec=group[0].start_sec,
                source_end_sec=group[-1].end_sec,
                evidence_segment_indices=[segment.segment_index for segment in group],
                tags=_make_tags(problem_slug, dominant_type, combined_text),
                transcript_excerpt=combined_text[:240],
            )
        )

    return units


def _should_split_group(
    current_group: list[TranscriptSegment],
    current_types: list[str],
    next_segment: TranscriptSegment,
    next_type: str,
) -> bool:
    """Decide whether to split before the next segment."""
    current_duration = current_group[-1].end_sec - current_group[0].start_sec
    current_chars = sum(len(segment.text) for segment in current_group)
    dominant = Counter(current_types).most_common(1)[0][0]

    transition_match = re.match(r"^(首先|然后|接下来|最后|再来|那么|所以)", next_segment.text)

    if current_duration >= 42 or current_chars >= 180:
        return True

    if transition_match and current_duration >= 8:
        return True

    if next_type in {"self_check", "exercise"} and next_type != dominant and current_duration >= 5:
        return True

    if next_type != dominant and current_duration >= 8 and current_chars >= 35:
        return True

    return False


def _dominant_type(texts: list[str]) -> str:
    """Vote on the dominant type for a group of texts."""
    votes = Counter(classify_text(text) for text in texts)
    return votes.most_common(1)[0][0]


def _make_summary(combined_text: str) -> str:
    """Generate a summary (first 90 chars, with punctuation normalized)."""
    compact = re.sub(r"[。！？!?.]+", "。", combined_text)
    summary = compact[:90].strip()
    return summary if len(compact) <= 90 else f"{summary}..."


def _make_title(
    problem_display_name: str,
    unit_type: str,
    sequence: int,
    combined_text: str,
) -> str:
    """Generate a knowledge unit title."""
    templates = {
        "explanation": "原理说明",
        "self_check": "自测与判断",
        "exercise": "改善动作",
        "cause": "影响与成因",
        "warning": "风险提醒",
    }
    focus = re.sub(r"[，。！？!?\s]+", "", combined_text)[:14]
    suffix = templates.get(unit_type, "内容整理")
    return f"{problem_display_name}{suffix} {sequence}: {focus}"


def _make_tags(problem_slug: str, unit_type: str, combined_text: str) -> list[str]:
    """Generate tags for search intent matching and filtering."""
    tags = [problem_slug, unit_type]

    if "头前" in combined_text or "头前移" in combined_text:
        tags.append("头前移")
    if "训练" in combined_text or "动作" in combined_text:
        tags.append("动作演示")
    if "自然放松" in combined_text or "判断" in combined_text:
        tags.append("自测")

    return sorted(set(tags))
