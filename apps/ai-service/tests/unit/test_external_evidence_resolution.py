from __future__ import annotations

from src.rag.external_evidence import build_claim_admissibility, resolve_external_reference


def _reference(url: str, *, scope: str = "section_direct") -> dict:
    return {
        "reference_id": "tfr-12345678901234567890123456789012",
        "label": "Source",
        "url": url,
        "scope": scope,
        "line": 42,
        "source_resolution_status": "unresolved",
        "support_status": "unreviewed",
    }


def test_pubmed_reference_gets_stable_canonical_identity_without_authority_promotion() -> None:
    resolved = resolve_external_reference(
        _reference("https://pubmed.ncbi.nlm.nih.gov/12345678/?utm_source=test#abstract")
    )

    assert resolved["provider"] == "pubmed"
    assert resolved["source_type"] == "bibliographic_index"
    assert resolved["canonical_key"] == "pmid:12345678"
    assert resolved["canonical_url"] == "https://pubmed.ncbi.nlm.nih.gov/12345678/"
    assert resolved["authority_tier"] == "unresolved"
    assert resolved["evidence_level"] == "unresolved"
    assert resolved["license_status"] == "unknown"
    assert resolved["support_status"] == "unreviewed"
    assert resolved["admissibility_status"] == "blocked"
    assert "support_unreviewed" in resolved["blocking_reasons"]


def test_doi_reference_normalizes_case_and_identity() -> None:
    resolved = resolve_external_reference(_reference("https://doi.org/10.2519/JOSPT.2020.9971"))

    assert resolved["provider"] == "doi"
    assert resolved["source_type"] == "doi_resolver"
    assert resolved["canonical_key"] == "doi:10.2519/jospt.2020.9971"
    assert resolved["canonical_url"] == "https://doi.org/10.2519/jospt.2020.9971"


def test_professional_organization_page_is_classified_but_not_auto_promoted() -> None:
    resolved = resolve_external_reference(
        _reference("https://www.iasp-pain.org/resources/terminology/?utm_medium=chat")
    )

    assert resolved["provider"] == "web"
    assert resolved["source_type"] == "professional_organization_page"
    assert resolved["canonical_url"] == "https://www.iasp-pain.org/resources/terminology/"
    assert resolved["authority_tier"] == "unresolved"
    assert resolved["admissibility_status"] == "blocked"


def test_claim_without_direct_reference_is_blocked() -> None:
    admissibility = build_claim_admissibility([])

    assert admissibility["status"] == "blocked"
    assert admissibility["publication_eligible"] is False
    assert admissibility["blocking_reasons"] == ["no_direct_external_reference"]


def test_direct_reference_still_requires_reviewed_support_and_governance() -> None:
    resolved = resolve_external_reference(
        _reference("https://www.iasp-pain.org/resources/terminology/")
    )
    admissibility = build_claim_admissibility([resolved])

    assert admissibility["status"] == "blocked"
    assert admissibility["publication_eligible"] is False
    assert "support_unreviewed" in admissibility["blocking_reasons"]
    assert "authority_unresolved" in admissibility["blocking_reasons"]
    assert "evidence_level_unresolved" in admissibility["blocking_reasons"]
    assert "license_unknown" in admissibility["blocking_reasons"]


def test_explicit_review_can_make_exact_relation_ready_for_claim_review(tmp_path) -> None:
    from src.rag.external_evidence import (
        apply_external_evidence_review,
        load_external_evidence_review,
    )

    review_path = tmp_path / "review.json"
    review_path.write_text(
        '''{
          "schema_version": "bodysense.external-evidence-review.v1",
          "review_id": "review-pilot",
          "snapshot_git_commit": "abc123",
          "reviewed_at": "2026-08-23T13:00:00Z",
          "sources": [{
            "canonical_key": "url:7091fe4bcd8c558fd8b4ae51682725bf",
            "canonical_url": "https://www.iasp-pain.org/resources/terminology/",
            "authority_tier": "B",
            "evidence_level": "professional_definition",
            "license_status": "citation_only",
            "review_status": "reviewed",
            "reviewed_by": "maintainer",
            "review_basis": "Official terminology page"
          }],
          "relations": [{
            "claim_id": "tfc-claim",
            "claim_content_hash": "aaaaaaaaaaaaaaaa",
            "canonical_key": "url:7091fe4bcd8c558fd8b4ae51682725bf",
            "support_status": "reviewed_support",
            "support_scope": "direct",
            "review_status": "reviewed"
          }]
        }''',
        encoding="utf-8",
    )
    manifest = load_external_evidence_review(review_path)
    resolved = resolve_external_reference(
        _reference("https://www.iasp-pain.org/resources/terminology/")
    )
    reviewed = apply_external_evidence_review(
        claim_id="tfc-claim",
        claim_content_hash="aaaaaaaaaaaaaaaa",
        resolved_references=[resolved],
        review_manifest=manifest,
    )
    admissibility = build_claim_admissibility(reviewed)

    assert reviewed[0]["authority_tier"] == "B"
    assert reviewed[0]["evidence_level"] == "professional_definition"
    assert reviewed[0]["license_status"] == "citation_only"
    assert reviewed[0]["support_status"] == "reviewed_support"
    assert reviewed[0]["admissibility_status"] == "admissible_for_claim_review"
    assert admissibility["evidence_ready_for_claim_review"] is True
    assert admissibility["publication_eligible"] is False
    assert admissibility["blocking_reasons"] == ["claim_review_unreviewed"]


def test_review_does_not_apply_to_different_claim_content_hash(tmp_path) -> None:
    from src.rag.external_evidence import (
        apply_external_evidence_review,
        load_external_evidence_review,
    )

    review_path = tmp_path / "review.json"
    review_path.write_text(
        '''{
          "schema_version": "bodysense.external-evidence-review.v1",
          "review_id": "review-pilot",
          "snapshot_git_commit": "abc123",
          "reviewed_at": "2026-08-23T13:00:00Z",
          "sources": [{
            "canonical_key": "url:7091fe4bcd8c558fd8b4ae51682725bf",
            "canonical_url": "https://www.iasp-pain.org/resources/terminology/",
            "authority_tier": "B",
            "evidence_level": "professional_definition",
            "license_status": "citation_only",
            "review_status": "reviewed",
            "reviewed_by": "maintainer",
            "review_basis": "Official terminology page"
          }],
          "relations": [{
            "claim_id": "tfc-claim",
            "claim_content_hash": "aaaaaaaaaaaaaaaa",
            "canonical_key": "url:7091fe4bcd8c558fd8b4ae51682725bf",
            "support_status": "reviewed_support",
            "support_scope": "direct",
            "review_status": "reviewed"
          }]
        }''',
        encoding="utf-8",
    )
    manifest = load_external_evidence_review(review_path)
    resolved = resolve_external_reference(
        _reference("https://www.iasp-pain.org/resources/terminology/")
    )
    reviewed = apply_external_evidence_review(
        claim_id="tfc-claim",
        claim_content_hash="bbbbbbbbbbbbbbbb",
        resolved_references=[resolved],
        review_manifest=manifest,
    )

    assert reviewed[0]["support_status"] == "unreviewed"
    assert reviewed[0]["admissibility_status"] == "blocked"
