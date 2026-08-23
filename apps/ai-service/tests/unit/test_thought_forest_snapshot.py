from __future__ import annotations

import json

import pytest
from pydantic import ValidationError

from src.rag.thought_forest_snapshot import build_generated_packs, load_thought_forest_snapshot


def _claim_candidate() -> dict:
    return {
        "claim_id": "tfc-12345678901234567890123456789012",
        "claim_kind": "definition",
        "candidate_scope": "section",
        "authority_tier": "C",
        "certainty": "unreviewed",
        "evidence_level": "unresolved",
        "external_evidence_status": "unresolved",
        "population": "unspecified",
    }


def _snapshot_payload(*, version: str = "bodysense.health.snapshot.v2") -> dict:
    section = {
        "section_key": "tfu-12345678901234567890123456789012",
        "title": "Definition",
        "heading_path": ["Sample", "Definition"],
        "line_start": 20,
        "line_end": 27,
        "markdown": "## Definition\n\nThis is the evidence paragraph.",
        "content_hash": "b" * 64,
    }
    if version == "bodysense.health.snapshot.v2":
        section["claim_candidate"] = _claim_candidate()
    return {
        "schema_version": version,
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
                "sections": [section],
            }
        ],
    }


def test_v2_snapshot_converts_claim_candidate_to_unpublished_pack(tmp_path) -> None:
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
    assert pack.units[0].metadata["claim_candidate"] == _claim_candidate()


def test_v1_snapshot_remains_backward_compatible(tmp_path) -> None:
    path = tmp_path / "snapshot-v1.json"
    path.write_text(
        json.dumps(_snapshot_payload(version="bodysense.health.snapshot.v1")),
        encoding="utf-8",
    )

    snapshot = load_thought_forest_snapshot(path)
    packs = build_generated_packs(snapshot)

    assert snapshot.schema_version == "bodysense.health.snapshot.v1"
    assert "claim_candidate" not in packs[0].units[0].metadata


def test_v2_snapshot_requires_claim_candidate(tmp_path) -> None:
    payload = _snapshot_payload()
    del payload["notes"][0]["sections"][0]["claim_candidate"]
    path = tmp_path / "snapshot.json"
    path.write_text(json.dumps(payload), encoding="utf-8")

    with pytest.raises(ValidationError, match="claim_candidate"):
        load_thought_forest_snapshot(path)


def test_snapshot_rejects_invalid_markdown_line_range(tmp_path) -> None:
    payload = _snapshot_payload()
    payload["notes"][0]["sections"][0]["line_start"] = 30
    payload["notes"][0]["sections"][0]["line_end"] = 20
    path = tmp_path / "snapshot.json"
    path.write_text(json.dumps(payload), encoding="utf-8")

    with pytest.raises(ValidationError, match="line_end"):
        load_thought_forest_snapshot(path)
