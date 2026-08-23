from __future__ import annotations

import json

import pytest
from pydantic import ValidationError

from src.rag.thought_forest_snapshot import build_generated_packs, load_thought_forest_snapshot


def _snapshot_payload() -> dict:
    return {
        "schema_version": "bodysense.health.snapshot.v1",
        "snapshot_id": "thought-forest:abc123:deadbeef0000",
        "generated_at": "2026-08-23T12:00:00Z",
        "authority_role": "curated_personal_knowledge",
        "repository": {"name": "thought-forest", "git_commit": "abc123"},
        "notes": [
            {
                "source_key": "thought-forest:z/sample.md",
                "source_type": "thought_forest_note",
                "path": "z/sample.md",
                "title": "Sample",
                "aliases": ["示例"],
                "description": "Sample description",
                "tags": ["life/health", "type/concept", "status/growing", "resource/assessment"],
                "note_type": "concept",
                "status": "growing",
                "updated": "2026-08-23T10:00:00Z",
                "problem_slug": "sample",
                "knowledge_kinds": ["assessment"],
                "content_hash": "a" * 64,
                "sections": [
                    {
                        "section_key": "tfu-12345678901234567890123456789012",
                        "title": "Definition",
                        "heading_path": ["Sample", "Definition"],
                        "line_start": 20,
                        "line_end": 27,
                        "markdown": "## Definition\n\nThis is the evidence paragraph.",
                        "content_hash": "b" * 64,
                    }
                ],
            }
        ],
    }


def test_snapshot_converts_to_unpublished_generated_pack(tmp_path) -> None:
    path = tmp_path / "snapshot.json"
    path.write_text(json.dumps(_snapshot_payload()), encoding="utf-8")

    snapshot = load_thought_forest_snapshot(path)
    packs = build_generated_packs(snapshot)

    assert len(packs) == 1
    pack = packs[0]
    assert pack.source.source_type == "thought_forest_note"
    assert pack.source.metadata["repository"]["git_commit"] == "abc123"
    assert pack.units[0].review_status == "generated"
    assert pack.units[0].source_start_sec == 0.0
    locator = pack.units[0].metadata["source_locator"]
    assert locator["locator_type"] == "markdown_lines"
    assert locator["path"] == "z/sample.md"
    assert locator["line_start"] == 20
    assert locator["line_end"] == 27
    assert locator["source_time_applicable"] is False
    assert pack.transcript_segments[0].metadata["source_locator"] == locator


def test_snapshot_rejects_invalid_markdown_line_range(tmp_path) -> None:
    payload = _snapshot_payload()
    payload["notes"][0]["sections"][0]["line_start"] = 30
    payload["notes"][0]["sections"][0]["line_end"] = 20
    path = tmp_path / "snapshot.json"
    path.write_text(json.dumps(payload), encoding="utf-8")

    with pytest.raises(ValidationError, match="line_end"):
        load_thought_forest_snapshot(path)
