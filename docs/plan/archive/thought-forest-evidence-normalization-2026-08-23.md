# Thought Forest Evidence Normalization

**Status:** Completed — 2026-08-23
**Date:** 2026-08-23

## Outcome

Promote the completed Thought Forest transport MVP into a governed evidence-identity vertical slice without publishing any personal synthesis.

```text
Thought Forest section
→ typed claim candidate
→ DB metadata
→ SearchResult source identity
→ normalized Agent Evidence
→ unpublished retrieval eval
```

## Current gap

- Snapshot v1 preserves Git/path/line provenance, but has no typed claim candidate contract.
- `KnowledgeLibrary.search()` drops `unit_key`, source type and metadata after retrieval.
- `normalize_evidence()` therefore still represents text evidence as `knowledge_unit + 00:00`.
- Thought Forest is Tier C personal synthesis and must not be silently promoted to primary evidence.

## Protected contracts

- Snapshot v1 remains readable by BodySense.
- Existing video ingestion and video evidence identity remain unchanged.
- `/api/knowledge/search` response shape remains unchanged.
- Thought Forest units remain generated/unpublished by default.
- No external citation is marked resolved in this phase.
- No production database writes.

## Phase 1 — Claim candidate contract

- Add snapshot v2 section-level `claim_candidate` metadata.
- Deterministically classify claim kind from heading + knowledge domain.
- Default governance metadata: authority tier C, evidence unresolved, certainty unreviewed.
- Keep section as the candidate scope; do not sentence-split human notes.

## Phase 2 — Evidence identity propagation

- Return stable unit/source identity and metadata from `KnowledgeLibrary.search()`.
- For Thought Forest evidence, derive `source_version` from Git commit + section content hash.
- Surface Markdown source locator and claim governance metadata in Agent evidence.
- Preserve current video normalization behavior.

## Phase 3 — Unpublished retrieval eval

- Add a small query set covering definition, interpretation boundary, assessment, safety, pain, differential, exercise and training dose.
- Add an eval runner using `include_unpublished=True` only.
- Report top-k problem recall and claim-kind hit rate.
- Do not change publication state based on this eval.

## Acceptance

1. Snapshot v2 exports every section with a stable claim candidate.
2. BodySense accepts v1 and v2; v2 rejects missing claim candidate metadata.
3. Thought Forest Agent evidence has stable source identity independent of DB row id.
4. Evidence includes Git/path/line locator plus Tier C / unresolved / unreviewed metadata.
5. Existing video evidence identity is unchanged by characterization test.
6. Focused and full AI-service tests pass.
7. Real unpublished eval runs against the pilot corpus and records measured results.


## Implementation Result

Completed vertical slice:

- Thought Forest snapshot schema advanced to `bodysense.health.snapshot.v2`.
- Every exported H2 section carries a stable section-scope `claim_candidate`.
- Governance defaults are conservative: `authority_tier=C`, `certainty=unreviewed`, `evidence_level=unresolved`, `external_evidence_status=unresolved`, `population=unspecified`.
- BodySense remains backward compatible with snapshot v1 and requires claim candidates for v2.
- `KnowledgeLibrary.search()` now preserves stable unit/source identity plus source/unit metadata internally, while the public knowledge API response contract remains unchanged.
- Thought Forest Agent evidence uses `source_key + unit_key` and `git_commit + section_content_hash`, so evidence identity is independent of transient database row ids.
- Markdown `repository/path/heading/line_start/line_end` provenance reaches normalized Agent evidence.
- Existing video evidence identity remains `knowledge_unit + database id + source timestamp` by characterization test.
- A source-type filter was added to the internal KnowledgeLibrary search boundary for isolated unpublished evaluation.
- Typed claim reranking is lexical-grounded and only acts as a light tie-breaker; it does not replace semantic/topic relevance.
- Pilot eval set: 12 unpublished Thought Forest queries covering definition, boundary, assessment, safety, pain, differential, intervention, dosage and biomechanics.
- Final pilot metrics at top-k=5: `top1_accuracy=0.6667`, `topk_problem_recall=1.0`, `claim_kind_hit_rate=1.0`.
- The gate is top-k problem recall + claim-kind coverage; top1 ranking remains a later retrieval-quality optimization and is not used to publish evidence.
- Complete AI-service unit suite passed: **311/311**. Ruff and `git diff --check` passed.
- Development PostgreSQL validation kept all Thought Forest units generated/unpublished; normal published-only search returned zero Thought Forest results.

No external citation was marked resolved and no Tier-C personal synthesis was published.

## Next Boundary

The next phase is external evidence resolution and admissibility:

```text
Tier-C claim candidate
→ citation/source extraction
→ canonical external source identity
→ authority/license/evidence metadata
→ claim-to-source support relation
→ review/admissibility decision
→ reviewed snapshot/publication candidate
```

Do not auto-promote a claim merely because an external URL or citation is detected.
