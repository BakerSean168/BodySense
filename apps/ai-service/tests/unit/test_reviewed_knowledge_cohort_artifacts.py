import json
from pathlib import Path

from src.rag.claim_review import ReviewedKnowledgeSnapshot, load_claim_review
from src.rag.external_evidence import load_external_evidence_review

REPO_ROOT = Path(__file__).parents[4]
EVIDENCE_DIR = REPO_ROOT / "docs" / "knowledges" / "evidence"


def test_pain_nociception_cohort_artifacts_are_exact_and_cross_bound() -> None:
    evidence = load_external_evidence_review(
        EVIDENCE_DIR / "external-evidence-review-pain-nociception-cohort.json"
    )
    claims = load_claim_review(EVIDENCE_DIR / "claim-review-pain-nociception-cohort.json")
    reviewed = ReviewedKnowledgeSnapshot.model_validate(
        json.loads(
            (EVIDENCE_DIR / "reviewed-knowledge-pain-nociception-cohort.json").read_text(
                encoding="utf-8"
            )
        )
    )

    assert evidence.snapshot_git_commit == claims.snapshot_git_commit == reviewed.source_git_commit
    assert claims.external_evidence_review_id == evidence.review_id
    assert reviewed.external_evidence_review_id == evidence.review_id
    assert reviewed.claim_review_id == claims.review_id
    assert reviewed.source_snapshot_id.startswith(f"thought-forest:{reviewed.source_git_commit}:")

    claim_decisions = {
        (item.unit_key, item.claim_id, item.claim_content_hash): item for item in claims.decisions
    }
    reviewed_units = {
        (item.unit_key, item.claim_id, item.claim_content_hash): item for item in reviewed.units
    }
    assert len(claim_decisions) == len(reviewed_units) == 3
    assert set(claim_decisions) == set(reviewed_units)
    assert all(item.decision == "approved" for item in claim_decisions.values())
    assert all(item.publication_eligible is True for item in reviewed_units.values())
    assert all(item.lifecycle_status == "reviewed" for item in reviewed_units.values())
    assert all(item.review_status == "reviewed" for item in reviewed_units.values())

    supported_claims = {relation.claim_id for relation in evidence.relations}
    assert supported_claims == {item.claim_id for item in claims.decisions}
    assert {source.canonical_key for source in evidence.sources} == {
        "url:7091fe4bcd8c558fd8b4ae51682725bf",
        "url:ab973d3f9c4bddfe9b6f4dffe11a5162",
    }
