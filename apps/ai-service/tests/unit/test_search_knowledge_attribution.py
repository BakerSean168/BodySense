from types import SimpleNamespace

import pytest

from src.services.agent.tools import search_knowledge


def _result(*, published: bool):
    return SimpleNamespace(
        id=1,
        title="疼痛与伤害感受 · 一句话定义",
        summary="疼痛与伤害感受不是同一现象",
        body_markdown="疼痛是不愉快的感觉与情绪体验。",
        source_title="Thought Forest",
        category="pain-science",
        source_type="thought_forest_note",
        source_key="thought-forest:z/pain-and-nociception.md",
        unit_key="tfu-example",
        lifecycle_status="published" if published else "reviewed",
        review_status="reviewed",
        publication_id="11111111-1111-1111-1111-111111111111" if published else "",
        publication_key="pain-definition-v3" if published else "",
        publication_batch_key="pain-batch" if published else "",
        published_version=3 if published else None,
        unit_metadata={
            "source_locator": {
                "locator_type": "markdown_lines",
                "repository": "thought-forest",
                "git_commit": "abc123",
                "path": "z/pain-and-nociception.md",
                "line_start": 20,
                "line_end": 23,
            },
            "claim_candidate": {"claim_id": "tfc-example", "claim_kind": "definition"},
            "claim_review": {"review_id": "review-example"},
        },
    )


@pytest.mark.asyncio
async def test_search_knowledge_exposes_published_evidence_ref_only_to_model(monkeypatch):
    class _Library:
        async def search(self, **_kwargs):
            return [_result(published=True)]

    monkeypatch.setattr(search_knowledge, "get_knowledge_library", lambda: _Library())
    output = await search_knowledge.handle_search_knowledge({"query": "什么是疼痛？"})

    assert output["has_results"] is True
    assert "Published Evidence Ref（仅供系统归因，不要展示给用户）" in output["result_text"]
    assert "published:11111111-1111-1111-1111-111111111111:v3:tfu-example" in output[
        "result_text"
    ]


@pytest.mark.asyncio
async def test_search_knowledge_does_not_mint_ref_for_unpublished_unit(monkeypatch):
    class _Library:
        async def search(self, **_kwargs):
            return [_result(published=False)]

    monkeypatch.setattr(search_knowledge, "get_knowledge_library", lambda: _Library())
    output = await search_knowledge.handle_search_knowledge({"query": "什么是疼痛？"})

    assert "Published Evidence Ref" not in output["result_text"]
