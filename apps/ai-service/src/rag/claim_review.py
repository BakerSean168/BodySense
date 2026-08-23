"""Explicit human claim-review contracts for governed Thought Forest knowledge."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

CLAIM_REVIEW_SCHEMA_VERSION = "bodysense.claim-review.v1"
REVIEWED_KNOWLEDGE_SNAPSHOT_SCHEMA_VERSION = "bodysense.reviewed-knowledge-snapshot.v1"


class ClaimReviewDecision(BaseModel):
    unit_key: str = Field(min_length=1)
    claim_id: str = Field(min_length=1)
    claim_content_hash: str = Field(min_length=16)
    decision: Literal["approved", "rejected"]
    review_status: Literal["reviewed"]
    reviewed_by: str = Field(min_length=1)
    review_basis: str = Field(min_length=1)
    quality_score: float = Field(ge=0.0, le=1.0)
    certainty: Literal["low", "moderate", "high"]
    population: str = Field(min_length=1)


class ClaimReviewManifest(BaseModel):
    schema_version: str
    review_id: str = Field(min_length=1)
    snapshot_git_commit: str = Field(min_length=1)
    external_evidence_review_id: str = Field(min_length=1)
    reviewed_at: str = Field(min_length=1)
    decisions: list[ClaimReviewDecision] = Field(min_length=1)

    @model_validator(mode="after")
    def validate_schema(self) -> "ClaimReviewManifest":
        if self.schema_version != CLAIM_REVIEW_SCHEMA_VERSION:
            raise ValueError(f"unsupported claim review schema: {self.schema_version}")
        identities = [
            (decision.unit_key, decision.claim_id, decision.claim_content_hash)
            for decision in self.decisions
        ]
        if len(identities) != len(set(identities)):
            raise ValueError("claim review manifest contains duplicate claim-version decisions")
        return self


class AppliedClaimReview(BaseModel):
    review_id: str
    decision: Literal["approved", "rejected"]
    review_status: Literal["reviewed"]
    reviewed_at: str
    reviewed_by: str
    review_basis: str
    quality_score: float
    certainty: Literal["low", "moderate", "high"]
    population: str
    external_evidence_review_id: str


class ReviewedKnowledgeUnit(BaseModel):
    unit_key: str
    claim_id: str
    claim_content_hash: str
    review_status: str
    lifecycle_status: str
    quality_score: float
    publication_eligible: bool
    source_locator: dict[str, Any]
    claim_review: dict[str, Any]


class ReviewedKnowledgeSnapshot(BaseModel):
    schema_version: str = REVIEWED_KNOWLEDGE_SNAPSHOT_SCHEMA_VERSION
    reviewed_snapshot_id: str
    source_snapshot_id: str
    source_git_commit: str
    external_evidence_review_id: str
    claim_review_id: str
    units: list[ReviewedKnowledgeUnit]


def load_claim_review(path: str | Path) -> ClaimReviewManifest:
    resolved = Path(path).resolve()
    payload = json.loads(resolved.read_text(encoding="utf-8"))
    return ClaimReviewManifest.model_validate(payload)


def apply_claim_review(
    *,
    unit_key: str,
    claim_id: str,
    claim_content_hash: str,
    claim_admissibility: dict[str, Any],
    review_manifest: ClaimReviewManifest | None,
) -> tuple[dict[str, Any], AppliedClaimReview | None, str, str, float]:
    """Apply an exact claim-version decision without performing publication."""
    if review_manifest is None:
        return claim_admissibility, None, "generated", "generated", 0.0

    decision = next(
        (
            item
            for item in review_manifest.decisions
            if item.unit_key == unit_key
            and item.claim_id == claim_id
            and item.claim_content_hash == claim_content_hash
        ),
        None,
    )
    if decision is None:
        return claim_admissibility, None, "generated", "generated", 0.0

    applied = AppliedClaimReview(
        review_id=review_manifest.review_id,
        decision=decision.decision,
        review_status=decision.review_status,
        reviewed_at=review_manifest.reviewed_at,
        reviewed_by=decision.reviewed_by,
        review_basis=decision.review_basis,
        quality_score=decision.quality_score,
        certainty=decision.certainty,
        population=decision.population,
        external_evidence_review_id=review_manifest.external_evidence_review_id,
    )

    if decision.decision == "rejected":
        rejected = {
            **claim_admissibility,
            "status": "claim_rejected",
            "publication_eligible": False,
            "blocking_reasons": ["claim_review_rejected"],
        }
        return rejected, applied, "rejected", "generated", 0.0

    if claim_admissibility.get("evidence_ready_for_claim_review") is not True:
        raise ValueError(
            f"claim review cannot approve {claim_id}: external evidence is not review-ready"
        )

    approved = {
        **claim_admissibility,
        "status": "claim_reviewed",
        "publication_eligible": True,
        "blocking_reasons": [],
        "claim_review_id": review_manifest.review_id,
    }
    return approved, applied, "reviewed", "reviewed", decision.quality_score


def build_reviewed_snapshot(
    *,
    source_snapshot_id: str,
    source_git_commit: str,
    external_evidence_review_id: str,
    claim_review_id: str,
    units: list[ReviewedKnowledgeUnit],
) -> ReviewedKnowledgeSnapshot:
    identity_payload = {
        "source_snapshot_id": source_snapshot_id,
        "source_git_commit": source_git_commit,
        "external_evidence_review_id": external_evidence_review_id,
        "claim_review_id": claim_review_id,
        "units": [unit.model_dump() for unit in units],
    }
    digest = hashlib.sha256(
        json.dumps(identity_payload, sort_keys=True, ensure_ascii=False).encode("utf-8")
    ).hexdigest()[:24]
    return ReviewedKnowledgeSnapshot(
        reviewed_snapshot_id=f"reviewed-knowledge:{digest}",
        source_snapshot_id=source_snapshot_id,
        source_git_commit=source_git_commit,
        external_evidence_review_id=external_evidence_review_id,
        claim_review_id=claim_review_id,
        units=units,
    )
