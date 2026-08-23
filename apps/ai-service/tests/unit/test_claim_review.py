from __future__ import annotations

import pytest

from src.rag.claim_review import (
    CLAIM_REVIEW_SCHEMA_VERSION,
    ClaimReviewManifest,
    apply_claim_review,
)


def _manifest(*, content_hash: str = "a" * 64, decision: str = "approved") -> ClaimReviewManifest:
    return ClaimReviewManifest.model_validate(
        {
            "schema_version": CLAIM_REVIEW_SCHEMA_VERSION,
            "review_id": "claim-review-pilot",
            "snapshot_git_commit": "abc123",
            "external_evidence_review_id": "external-review-pilot",
            "reviewed_at": "2026-08-23T21:50:00+08:00",
            "decisions": [
                {
                    "unit_key": "tfu-pilot",
                    "claim_id": "tfc-pilot",
                    "claim_content_hash": content_hash,
                    "decision": decision,
                    "review_status": "reviewed",
                    "reviewed_by": "maintainer",
                    "review_basis": (
                        "Exact claim wording reviewed against admitted external evidence."
                    ),
                    "quality_score": 0.95,
                    "certainty": "high",
                    "population": "general",
                }
            ],
        }
    )


def _ready() -> dict:
    return {
        "status": "evidence_ready_for_claim_review",
        "evidence_ready_for_claim_review": True,
        "publication_eligible": False,
        "blocking_reasons": ["claim_review_unreviewed"],
        "direct_reference_count": 1,
        "admissible_reference_count": 1,
    }


def test_approved_exact_claim_becomes_reviewed_but_not_published() -> None:
    admissibility, applied, review_status, lifecycle_status, quality_score = apply_claim_review(
        unit_key="tfu-pilot",
        claim_id="tfc-pilot",
        claim_content_hash="a" * 64,
        claim_admissibility=_ready(),
        review_manifest=_manifest(),
    )

    assert admissibility["publication_eligible"] is True
    assert admissibility["status"] == "claim_reviewed"
    assert applied is not None and applied.decision == "approved"
    assert review_status == "reviewed"
    assert lifecycle_status == "reviewed"
    assert quality_score == 0.95


def test_stale_claim_hash_does_not_reuse_review() -> None:
    admissibility, applied, review_status, lifecycle_status, quality_score = apply_claim_review(
        unit_key="tfu-pilot",
        claim_id="tfc-pilot",
        claim_content_hash="b" * 64,
        claim_admissibility=_ready(),
        review_manifest=_manifest(content_hash="a" * 64),
    )

    assert admissibility["publication_eligible"] is False
    assert applied is None
    assert review_status == "generated"
    assert lifecycle_status == "generated"
    assert quality_score == 0.0


def test_approval_fails_closed_without_review_ready_evidence() -> None:
    with pytest.raises(ValueError, match="external evidence is not review-ready"):
        apply_claim_review(
            unit_key="tfu-pilot",
            claim_id="tfc-pilot",
            claim_content_hash="a" * 64,
            claim_admissibility={
                "status": "blocked",
                "evidence_ready_for_claim_review": False,
                "publication_eligible": False,
                "blocking_reasons": ["support_unreviewed"],
            },
            review_manifest=_manifest(),
        )


def test_rejected_claim_remains_unpublishable() -> None:
    admissibility, applied, review_status, lifecycle_status, quality_score = apply_claim_review(
        unit_key="tfu-pilot",
        claim_id="tfc-pilot",
        claim_content_hash="a" * 64,
        claim_admissibility=_ready(),
        review_manifest=_manifest(decision="rejected"),
    )

    assert admissibility["publication_eligible"] is False
    assert admissibility["blocking_reasons"] == ["claim_review_rejected"]
    assert applied is not None and applied.decision == "rejected"
    assert review_status == "rejected"
    assert lifecycle_status == "generated"
    assert quality_score == 0.0
