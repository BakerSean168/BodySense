"""Fallback reply construction and citation event emission.

These helpers are used by the active consultation runtime
(`runtime/consultation_thread.py`) when no online LLM is configured or when
knowledge search results need to be surfaced as citation events. They are pure
functions with no dependency on the LangGraph runtime.
"""

from __future__ import annotations

import re
from typing import Any, Callable, Protocol


class StreamWriter(Protocol):
    def __call__(self, event: dict[str, Any]) -> None: ...


def build_fallback_reply(
    user_message: str,
    rag_results: list[dict[str, Any]] | None = None,
) -> str:
    """Build a deterministic reply when no online LLM is configured."""
    if not rag_results:
        return (
            "我已经收到你的描述，但当前本地环境没有配置云端大模型，且知识库里暂时没有检索到足够匹配的条目。\n"
            "你可以继续补充具体部位、动作场景、是否双侧对称，以及持续多久，我会继续按本地知识库帮你缩小范围。"
        )

    top = rag_results[0]
    title = top.get("title", "相关体态问题")
    summary = top.get("summary", "").strip()
    content = top.get("body_markdown") or top.get("content") or summary or ""
    plain = _markdown_to_text(content)
    lines = [f'根据当前本地知识库，你提到的问题最接近"{title}"。']

    if summary:
        lines.append(f"核心判断：{summary}")
    if plain:
        lines.append(f"知识要点：{plain[:280]}")

    clips = top.get("clips") or []
    if clips:
        clip_titles = [c.get("title", "").strip() for c in clips[:2] if c.get("title")]
        if clip_titles:
            lines.append(f"可参考的动作演示：{'、'.join(clip_titles)}。")

    if len(rag_results) > 1:
        extra = [r.get("title", "").strip() for r in rag_results[1:3] if r.get("title")]
        if extra:
            lines.append(f"我同时参考了：{'、'.join(extra)}。")

    lines.append("当前回答来自本地 curated 知识库整理，不构成医疗诊断；")
    lines.append("如果你愿意，我可以继续根据你的具体症状帮你细化判断。")
    return "\n".join(lines)


def emit_citation_events(search_results: list[Any], writer: Callable[[Any], None]) -> None:
    """Emit NDJSON citation events for knowledge search results."""
    for result in search_results:
        body = getattr(result, "body_markdown", "") or ""
        writer(
            {
                "type": "citation",
                "citation": {
                    "title": getattr(result, "title", ""),
                    "summary": getattr(result, "summary", ""),
                    "body_markdown": body[:500] if len(body) > 500 else body,
                    "source_title": getattr(result, "source_title", ""),
                    "source_author": getattr(result, "source_author", ""),
                    "category": getattr(result, "category", ""),
                    "problem_slug": getattr(result, "problem_slug", ""),
                    "unit_type": getattr(result, "unit_type", ""),
                    "tags": getattr(result, "tags", []) or [],
                    "clips": getattr(result, "clips", []) or [],
                },
            }
        )


def _markdown_to_text(content: str) -> str:
    text = re.sub(r"^#+\s*", "", content, flags=re.MULTILINE)
    text = re.sub(r"^[*-]\s*", "", text, flags=re.MULTILINE)
    text = re.sub(r"\n{2,}", "\n", text)
    return text.strip()
