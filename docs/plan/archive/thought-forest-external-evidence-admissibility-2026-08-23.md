# Thought Forest External Evidence Resolution + Admissibility

**Status:** Active
**Date:** 2026-08-23

## Outcome

Extend the governed Thought Forest knowledge path from typed Tier-C claim candidates to conservative external-evidence reference candidates without implying that a link proves a claim.

```text
Thought Forest section
→ direct URL reference candidate / note bibliography candidate
→ canonical external identity
→ unresolved authority / evidence / license / support
→ claim admissibility = blocked by default
```

## Protected contracts

- Snapshot v1/v2 remain readable by BodySense.
- Snapshot v3 does not promote Tier-C claims automatically.
- Note-level bibliography references MUST NOT be copied onto every claim as support.
- Existing video evidence identity and online published-only retrieval remain unchanged.
- No production DB ingestion or publication in this phase.
- Network metadata fetching is out of scope for the first vertical slice; resolution is deterministic identity normalization only.

## Tickets

### EXT-EVID-01 — Export scoped reference candidates

- Add snapshot v3.
- Extract Markdown/raw URL references with absolute Markdown line provenance.
- Distinguish `section_direct` from `note_bibliography`.
- Add explicit inline IASP citations to the pilot pain-science note.

### EXT-EVID-02 — Resolve canonical external identities

- Canonicalize PubMed, DOI and generic HTTPS references.
- Classify only source shape/provider, not evidence authority.
- Preserve `authority_tier=unresolved`, `evidence_level=unresolved`, `license_status=unknown`, `support_status=unreviewed`.

### EXT-EVID-03 — Enforce conservative claim admissibility

- Direct external refs still do not imply support.
- Missing direct refs and unresolved governance block publication eligibility.
- Store resolution/admissibility metadata through KnowledgeLibrary.
- Propagate to normalized Agent evidence for future review/eval.

### EXT-EVID-04 — Vertical verification

- Real snapshot export from committed Thought Forest.
- Dry-run + development PostgreSQL ingestion.
- Verify bibliography is not sprayed across claims.
- Verify IASP direct references become stable canonical candidates but remain blocked.
- Full AI-service unit suite + CI before merge.

## Non-goals

- No full-text scraping.
- No automatic PubMed/DOI metadata download.
- No automatic evidence-level inference from article titles.
- No human-review UI.
- No publication batch.

## Implementation Result

Completed 2026-08-23.

Verified vertical evidence:

- Thought Forest snapshot advanced to `bodysense.health.snapshot.v3` while BodySense retains v1/v2 compatibility.
- Export preserves `section_direct` references separately from `note_bibliography`; bibliography references are never sprayed across claims.
- Pilot snapshot from Thought Forest commit `4990c5844392ec7c36ff217e3e66ac3ceff1ac9a`: 11 notes / 115 units / 3 direct references / 23 bibliography references.
- Deterministic resolver canonicalizes PubMed (`pmid:*`), DOI (`doi:*`) and generic HTTPS (`url:*`) identities without network access or authority inference.
- Unreviewed external references remain `authority_tier=unresolved`, `evidence_level=unresolved`, `license_status=unknown`, `support_status=unreviewed` and blocked.
- Explicit review manifest is bound to `snapshot_git_commit + claim_id + claim_content_hash + canonical_key`; stale claim content cannot inherit support review.
- IASP Pain definition pilot is manually governed as Tier B / `professional_definition` / `citation_only` / `reviewed_support`.
- That pilot advances only to `evidence_ready_for_claim_review`; `publication_eligible` remains false because claim review/publication is a later phase.
- Without explicit review: 115/115 claims blocked, 0 evidence-ready, 0 publication eligible.
- With explicit pilot review: 114 blocked, 1 evidence-ready, 0 publication eligible.
- Development PostgreSQL v49 persisted reviewed source/support metadata and Markdown provenance; default published-only retrieval still returned 0 Thought Forest results.
- Full AI-service Ruff passed and unit suite passed: **321/321**.
- Temporary development PostgreSQL was stopped after verification.

## Follow-up Boundary

Next phase is claim review + publication governance, not more automatic source promotion:

```text
evidence-ready Tier-C claim
→ human claim review
→ quality / wording / population / support-scope decision
→ reviewed knowledge snapshot
→ publication batch
→ rollback + production retrieval/grounding eval
```

The external evidence resolver must remain conservative; network metadata fetching and additional source reviews can be added independently without bypassing review relations.
