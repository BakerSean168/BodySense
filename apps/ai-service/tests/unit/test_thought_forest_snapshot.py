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
    if version in {"bodysense.health.snapshot.v2", "bodysense.health.snapshot.v3"}:
        section["claim_candidate"] = _claim_candidate()
    if version == "bodysense.health.snapshot.v3":
        section["evidence_reference_candidates"] = [
            {
                "reference_id": "tfr-12345678901234567890123456789012",
                "label": "IASP",
                "url": "https://www.iasp-pain.org/resources/terminology/",
                "scope": "section_direct",
                "line": 24,
                "source_resolution_status": "unresolved",
                "support_status": "unreviewed",
            }
        ]
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
                **(
                    {
                        "bibliography_reference_candidates": [
                            {
                                "reference_id": "tfr-bibliography-12345678901234567890",
                                "label": "PubMed",
                                "url": "https://pubmed.ncbi.nlm.nih.gov/12345678/",
                                "scope": "note_bibliography",
                                "line": 120,
                                "source_resolution_status": "unresolved",
                                "support_status": "unreviewed",
                            }
                        ]
                    }
                    if version == "bodysense.health.snapshot.v3"
                    else {}
                ),
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


def test_v3_snapshot_resolves_direct_references_without_spraying_bibliography(tmp_path) -> None:
    path = tmp_path / "snapshot-v3.json"
    path.write_text(
        json.dumps(_snapshot_payload(version="bodysense.health.snapshot.v3")),
        encoding="utf-8",
    )

    snapshot = load_thought_forest_snapshot(path)
    pack = build_generated_packs(snapshot)[0]
    unit_metadata = pack.units[0].metadata

    assert snapshot.schema_version == "bodysense.health.snapshot.v3"
    assert len(unit_metadata["external_evidence_candidates"]) == 1
    direct = unit_metadata["external_evidence_candidates"][0]
    assert direct["source_type"] == "professional_organization_page"
    assert direct["relation_scope"] == "section_direct"
    assert direct["support_status"] == "unreviewed"
    assert unit_metadata["claim_admissibility"]["publication_eligible"] is False
    assert len(pack.source.metadata["bibliography_reference_candidates"]) == 1
    bibliography = pack.source.metadata["bibliography_reference_candidates"]
    assert bibliography[0]["canonical_key"] == "pmid:12345678"
    assert all(
        item["canonical_key"] != "pmid:12345678"
        for item in unit_metadata["external_evidence_candidates"]
    )


def test_v3_snapshot_requires_scoped_reference_fields(tmp_path) -> None:
    payload = _snapshot_payload(version="bodysense.health.snapshot.v3")
    del payload["notes"][0]["sections"][0]["evidence_reference_candidates"]
    path = tmp_path / "snapshot-v3-invalid.json"
    path.write_text(json.dumps(payload), encoding="utf-8")

    with pytest.raises(ValidationError, match="evidence_reference_candidates"):
        load_thought_forest_snapshot(path)


def test_v3_review_manifest_must_match_snapshot_commit(tmp_path) -> None:
    from src.rag.external_evidence import ExternalEvidenceReviewManifest

    snapshot_path = tmp_path / "snapshot-v3.json"
    snapshot_path.write_text(
        json.dumps(_snapshot_payload(version="bodysense.health.snapshot.v3")),
        encoding="utf-8",
    )
    snapshot = load_thought_forest_snapshot(snapshot_path)
    review = ExternalEvidenceReviewManifest.model_validate(
        {
            "schema_version": "bodysense.external-evidence-review.v1",
            "review_id": "stale-review",
            "snapshot_git_commit": "different-commit",
            "reviewed_at": "2026-08-23T13:00:00Z",
            "sources": [],
            "relations": [],
        }
    )

    with pytest.raises(ValueError, match="snapshot_git_commit"):
        build_generated_packs(snapshot, review_manifest=review)


def test_v3_claim_review_marks_only_exact_reviewed_unit_as_reviewed(tmp_path) -> None:
    from src.rag.claim_review import ClaimReviewManifest
    from src.rag.external_evidence import ExternalEvidenceReviewManifest

    snapshot_path = tmp_path / "snapshot-v3.json"
    snapshot_path.write_text(
        json.dumps(_snapshot_payload(version="bodysense.health.snapshot.v3")),
        encoding="utf-8",
    )
    snapshot = load_thought_forest_snapshot(snapshot_path)
    canonical_key = "url:7091fe4bcd8c558fd8b4ae51682725bf"
    evidence_review = ExternalEvidenceReviewManifest.model_validate(
        {
            "schema_version": "bodysense.external-evidence-review.v1",
            "review_id": "external-review",
            "snapshot_git_commit": "abc123",
            "reviewed_at": "2026-08-23T13:00:00Z",
            "sources": [
                {
                    "canonical_key": canonical_key,
                    "canonical_url": "https://www.iasp-pain.org/resources/terminology/",
                    "authority_tier": "B",
                    "evidence_level": "professional_definition",
                    "license_status": "citation_only",
                    "review_status": "reviewed",
                    "reviewed_by": "maintainer",
                    "review_basis": "Official professional definition.",
                }
            ],
            "relations": [
                {
                    "claim_id": _claim_candidate()["claim_id"],
                    "claim_content_hash": "b" * 64,
                    "canonical_key": canonical_key,
                    "support_status": "reviewed_support",
                    "support_scope": "direct",
                    "review_status": "reviewed",
                }
            ],
        }
    )
    claim_review = ClaimReviewManifest.model_validate(
        {
            "schema_version": "bodysense.claim-review.v1",
            "review_id": "claim-review",
            "snapshot_git_commit": "abc123",
            "external_evidence_review_id": "external-review",
            "reviewed_at": "2026-08-23T13:10:00Z",
            "decisions": [
                {
                    "unit_key": "tfu-12345678901234567890123456789012",
                    "claim_id": _claim_candidate()["claim_id"],
                    "claim_content_hash": "b" * 64,
                    "decision": "approved",
                    "review_status": "reviewed",
                    "reviewed_by": "maintainer",
                    "review_basis": "Claim wording reviewed against the admitted source.",
                    "quality_score": 0.95,
                    "certainty": "high",
                    "population": "general",
                }
            ],
        }
    )

    unit = build_generated_packs(
        snapshot,
        review_manifest=evidence_review,
        claim_review_manifest=claim_review,
    )[0].units[0]

    assert unit.review_status == "reviewed"
    assert unit.lifecycle_status == "reviewed"
    assert unit.quality_score == 0.95
    assert unit.content_hash == "b" * 64
    assert unit.metadata["claim_review"]["decision"] == "approved"
    assert unit.metadata["claim_admissibility"]["publication_eligible"] is True
