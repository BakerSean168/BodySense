from __future__ import annotations

from src.agents.evidence import normalize_evidence
from src.rag.knowledge_library import SearchResult


def _thought_forest_result(*, row_id: int) -> SearchResult:
    locator = {
        "locator_type": "markdown_lines",
        "repository": "thought-forest",
        "git_commit": "abc123",
        "path": "z/sample.md",
        "line_start": 20,
        "line_end": 27,
        "heading_path": ["Sample", "Definition"],
        "source_time_applicable": False,
    }
    return SearchResult(
        id=row_id,
        unit_key="tfu-12345678901234567890123456789012",
        problem_slug="sample",
        category="assessment",
        unit_type="reference",
        title="Sample · Definition",
        summary="Summary",
        body_markdown="Body",
        similarity=0.9,
        source_key="thought-forest:z/sample.md",
        source_type="thought_forest_note",
        source_title="Sample",
        source_author="Thought Forest",
        source_start_sec=0.0,
        source_end_sec=0.0,
        tags=["life/health"],
        unit_metadata={
            "source_locator": locator,
            "section_content_hash": "b" * 64,
            "claim_candidate": {
                "claim_id": "tfc-12345678901234567890123456789012",
                "claim_kind": "definition",
                "authority_tier": "C",
                "certainty": "unreviewed",
                "evidence_level": "unresolved",
                "external_evidence_status": "unresolved",
                "population": "unspecified",
            },
        },
        source_metadata={"repository": {"name": "thought-forest", "git_commit": "abc123"}},
    )


def test_thought_forest_evidence_identity_uses_git_and_content_not_db_row_id() -> None:
    first = normalize_evidence("user-1", _thought_forest_result(row_id=10))
    second = normalize_evidence("user-1", _thought_forest_result(row_id=999))

    assert first["evidence_id"] == second["evidence_id"]
    assert first["source_type"] == "thought_forest_note"
    assert first["source_key"] == (
        "thought-forest:z/sample.md#tfu-12345678901234567890123456789012"
    )
    assert first["source_version"] == f"abc123:{'b' * 64}"
    assert first["source_locator"]["line_start"] == 20
    assert first["claim_kind"] == "definition"
    assert first["authority_tier"] == "C"
    assert first["evidence_level"] == "unresolved"
    assert first["certainty"] == "unreviewed"
    assert first["external_evidence_status"] == "unresolved"


def test_video_evidence_identity_remains_backward_compatible() -> None:
    result = SearchResult(
        id=42,
        unit_key="video-unit-42",
        problem_slug="shoulder",
        category="exercise",
        unit_type="exercise",
        title="Video unit",
        summary="Summary",
        body_markdown="Body",
        similarity=0.8,
        source_key="video:source",
        source_type="video",
        source_title="Video",
        source_author="Author",
        source_start_sec=12.0,
        source_end_sec=15.0,
    )

    evidence = normalize_evidence("user-1", result)

    assert evidence["source_type"] == "knowledge_unit"
    assert evidence["source_key"] == "knowledge-unit:42"
    assert evidence["source_version"] == "00:12-00:15"
    assert "source_locator" not in evidence
