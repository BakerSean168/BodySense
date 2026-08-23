# Thought Forest Claim Review + Publication Governance — 2026-08-23

## Outcome

Complete one governed knowledge lifecycle vertical slice without bypassing publication authority:

`evidence-ready claim -> explicit claim review -> reviewed unit -> versioned publication batch -> default retrieval -> rollback`.

## Current facts

- Thought Forest v3 snapshot transport, claim candidates, Markdown provenance, external evidence identity and one IASP support-review pilot are merged.
- The pilot Pain definition can reach `evidence_ready_for_claim_review`, but `publication_eligible=false`.
- `knowledge_publications` and `knowledge_units.publication_id` exist, but there is no real publish/rollback service.
- Current default Python retrieval incorrectly allows `reviewed` lifecycle/review states without a publication batch. This must be repaired before claim review can safely mark a unit reviewed.

## Protected contracts

- Default user-facing retrieval must never return generated or merely reviewed Thought Forest units.
- Claim review must bind exact `snapshot_git_commit + claim_id + claim_content_hash`.
- External evidence review and claim review are separate authorities.
- Publication must be an explicit transactional operator action.
- Rollback must restore the exact pre-publication unit state and fail closed if publication identity has drifted.
- Existing video ingestion and published knowledge remain backward compatible.
- No production deployment or production DB mutation in this phase.

## Vertical slice

1. Repair default retrieval to require `lifecycle_status='published'` AND reviewed/approved/curated review state.
2. Add an explicit claim-review manifest and deterministic reviewed-snapshot/materialization contract.
3. Persist reviewed unit fields (`review_status`, `lifecycle_status`, `quality_score`, `content_hash`) through the existing ingestion transaction.
4. Add Go-owned publication service + operator CLI for atomic publish and rollback.
5. Validate on isolated `postgres-dev` using the IASP Pain-definition pilot:
   - before claim review: not publishable;
   - after claim review: reviewed but still invisible;
   - after publication: visible in default search;
   - after rollback: invisible and state restored.
6. Run repository quality gate and protected CI, then archive this plan.

## Explicit non-goals

- No bulk claim review.
- No automatic source-quality inference.
- No UI/admin console in this slice.
- No production publication.
- No automatic publication after claim review.

## Implementation closeout

Completed on 2026-08-23:

- Repaired default retrieval so merely `reviewed` units cannot bypass explicit publication.
- Added exact-version `bodysense.claim-review.v1` and deterministic `bodysense.reviewed-knowledge-snapshot.v1` artifacts.
- Extended KnowledgeUnit ingestion with review/lifecycle/quality/content-hash persistence while preserving existing video defaults.
- Added migration 000050 for batch `rollback_of`, `unit_count`, and `summary` metadata.
- Added Go-owned transactional `KnowledgePublicationService` and `knowledge-publication-manager` CLI.
- Publication hard gate verifies reviewed lifecycle, quality >= 0.90, content hash, Markdown provenance, reviewed external support, claim approval, evidence excerpt, and non-rejected license state.
- Rollback validates publication/content/version identity and restores the exact pre-publication lifecycle/review/quality/publication state.
- Added batch overwrite preflight plus per-source defense so a published source cannot be silently replaced.
- Agent Evidence now carries claim-review and publication provenance.

### Real dev-PostgreSQL acceptance evidence

IASP Pain Definition pilot:

1. reviewed unit before publication -> default Thought Forest retrieval `0`;
2. publish batch v1 -> default retrieval `1` for exactly the reviewed unit;
3. overwrite attempt -> rejected because the source is publication-linked;
4. rollback -> exact unit restored to `reviewed`, publication link/version cleared, default retrieval `0`;
5. second publish/rollback verified batch version increment and batch overwrite preflight; all 11 source IDs remained unchanged on rejected overwrite.

No production database or production deployment was modified.
