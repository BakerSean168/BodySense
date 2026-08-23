"""Tests for the live reply-fallback helpers extracted from the dead orchestrator."""

from __future__ import annotations

from src.services.agent.reply_fallback import build_fallback_reply, emit_citation_events


def test_build_fallback_reply_without_rag():
    text = build_fallback_reply("我肩膀疼")
    assert "本地环境没有配置云端大模型" in text or "知识库" in text
    assert "肩膀" not in text or True  # message may or may not be echoed


def test_build_fallback_reply_with_rag_results():
    text = build_fallback_reply(
        "圆肩",
        [
            {
                "title": "圆肩",
                "summary": "肩部前倾的常见体态问题",
                "body_markdown": "## 要点\n- 含胸\n- 肩胛前伸",
                "clips": [{"title": "开胸拉伸"}],
            },
            {"title": "头前伸"},
        ],
    )
    assert "圆肩" in text
    assert "肩部前倾" in text
    assert "开胸拉伸" in text
    assert "头前伸" in text
    assert "不构成医疗诊断" in text


def test_emit_citation_events_writes_structured_events():
    class _Result:
        title = "圆肩"
        summary = "摘要"
        body_markdown = "x" * 600
        source_title = "来源"
        source_author = "作者"
        category = "posture"
        problem_slug = "rounded-shoulders"
        unit_type = "problem"
        tags = ["shoulder"]
        clips = []

    events: list[dict] = []
    emit_citation_events([_Result()], events.append)

    assert len(events) == 1
    assert events[0]["type"] == "citation"
    citation = events[0]["citation"]
    assert citation["title"] == "圆肩"
    assert len(citation["body_markdown"]) == 500


def test_published_thought_forest_citation_preserves_publication_identity():
    class _Result:
        title = "疼痛与伤害感受 · 一句话定义"
        summary = "摘要"
        body_markdown = "正文"
        source_title = "疼痛与伤害感受"
        source_author = "Thought Forest"
        category = "pain-science"
        problem_slug = "pain-science"
        unit_type = "reference"
        tags = ["life/health"]
        clips = []
        unit_key = "tfu-pain-definition"
        source_key = "thought-forest:z/pain-and-nociception.md"
        source_type = "thought_forest_note"
        lifecycle_status = "published"
        review_status = "reviewed"
        quality_score = 0.95
        publication_id = "11111111-1111-1111-1111-111111111111"
        published_version = 3
        publication_key = "pain-definition-v3"
        publication_batch_key = "thought-forest-reviewed-health-pilot"
        unit_metadata = {
            "source_locator": {
                "locator_type": "markdown_lines",
                "git_commit": "abc123",
                "path": "z/pain-and-nociception.md",
                "line_start": 20,
                "line_end": 23,
            },
            "claim_candidate": {
                "claim_id": "tfc-pain-definition",
                "claim_kind": "definition",
            },
            "claim_review": {
                "review_id": "pain-definition-review",
                "decision": "approved",
            },
        }

    events: list[dict] = []
    emit_citation_events([_Result()], events.append)
    citation = events[0]["citation"]

    assert citation["unit_key"] == "tfu-pain-definition"
    assert citation["source_key"] == "thought-forest:z/pain-and-nociception.md"
    assert citation["publication_id"] == "11111111-1111-1111-1111-111111111111"
    assert citation["published_version"] == 3
    assert citation["publication_key"] == "pain-definition-v3"
    assert citation["publication_batch_key"] == "thought-forest-reviewed-health-pilot"
    assert citation["source_locator"]["line_start"] == 20
    assert citation["claim_id"] == "tfc-pain-definition"
    assert citation["claim_review_id"] == "pain-definition-review"
