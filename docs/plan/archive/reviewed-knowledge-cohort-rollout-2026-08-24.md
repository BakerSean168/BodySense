# Reviewed Knowledge Cohort Rollout — 2026-08-24

## Outcome

Expand the single Pain Definition pilot into the first bounded reviewed/published cohort without weakening any Stage 4–7 governance boundary:

```text
Thought Forest v3 snapshot
→ exact direct external support review
→ exact claim review
→ immutable reviewed snapshot (cohort artifact)
→ artifact-bound publication
→ published-only cohort regression
→ runtime_answer observation
→ continue / hold / rollback recommendation
```

The first cohort is deliberately small: three low-risk Pain/Nociception definition or interpretation-boundary units backed directly by official IASP pages.

## Protected contracts

- A new Thought Forest Git commit creates a new snapshot identity. Previous pilot review approval does not inherit across snapshot_git_commit.
- Human notes remain readable H2-level concepts; no sentence-level RAG fragmentation.
- External URLs do not imply support. Exact source + claim-version relations remain required.
- `reviewed` never implies `published`; publication remains an explicit Go-owned transition.
- Default retrieval remains published-only and quality-gated.
- Stage 7 answer attribution/runtime observations remain additive and publication-version pinned.
- Existing `publish`/`rollback` CLI and publication records remain backward compatible.
- No new DB migration is required unless evidence proves the existing JSONB publication metadata cannot carry cohort identity.
- No production knowledge publish occurs until the merged code/version and production migration/deployment state are explicitly verified.

## S8-010 — Create a new immutable Pain/Nociception source snapshot

**Goal:** Produce a new governed snapshot with direct IASP support on three bounded sections.

**Implementation:**
- Use self-describing H2 claim scopes: `疼痛（Pain）是什么`, `疼痛（Pain）与伤害感受（Nociception）的核心解释边界`, and `伤害感受（Nociception）是什么`.
- Bind Pain/Nociception definitions directly to IASP Terminology.
- Bind the core interpretation boundary directly to IASP Revised Definition + Terminology.
- Export from a committed Thought Forest revision using the existing v3 exporter.

**Acceptance:** snapshot schema stays v3; source Git commit is immutable; exactly the intended sections carry direct references; unrelated sections remain unreviewed.

## S8-020 — Review the three-claim cohort

**Goal:** Create exact external-evidence review, claim-review, and reviewed-snapshot artifacts for the new source revision.

**Acceptance:**
- review manifests bind exact `snapshot_git_commit + claim_id + claim_content_hash + canonical_key`;
- only the three intended units become `reviewed` and `publication_eligible=true`;
- all remaining units remain generated/unpublished;
- reviewed snapshot contains exactly three units and preserves Markdown provenance.

## S8-030 — Artifact-bound publication

**Goal:** Prevent controlled rollout operators from hand-assembling a different unit set than the reviewed cohort artifact.

**Implementation:** add a `publish-reviewed` publication-manager path that loads `bodysense.reviewed-knowledge-snapshot.v1`, validates the artifact, publishes exactly its eligible unit set, and persists reviewed snapshot/source/evidence-review/claim-review identity in publication JSONB metadata. Existing manual `publish` remains available for compatibility/testing.

**Acceptance:** artifact unit drift, duplicate unit identity, non-eligible unit, or DB/artifact content/review identity mismatch fails closed before publication.

## S8-040 — Cohort published regression and observation gate

**Goal:** Expand the Stage 6 published-only eval from one unit to the three-unit cohort.

**Acceptance:** positive cases retrieve the expected exact unit/claim; negative unpublished topics return no Thought Forest result; publication leakage remains zero; eval observations attach to the exact publication/version and produce deterministic `continue/hold/rollback` semantics.

## S8-050 — Real PostgreSQL vertical verification

**Goal:** Run the complete cohort lifecycle on isolated GCP-dev PostgreSQL.

**Acceptance:** ingest 115-ish generated units from the new snapshot; exactly three reviewed units exist; default retrieval is empty before publish; artifact-bound publish exposes exactly the cohort; cohort eval passes; observation gate is deterministic; rollback restores all three units to reviewed/unpublished; source overwrite protection remains atomic.

## S8-060 — Release closure

**Goal:** Merge the implementation without disturbing the concurrent security closeout worktree.

**Acceptance:** focused tests → package checks → `bash scripts/validate-repo.sh` → protected PR checks → normal merge. Archive this plan only after the implementation and dev vertical slice pass. Production publication is a separate explicit release action after deployment-state verification.
## Implementation checkpoint

- Thought Forest source commit: `8dbe766899da073727336a6f93cb142e34eeb4e8`; 11 notes / 115 sections; source-side exporter tests and changed-only KB gate pass locally.
- Reviewed cohort artifact: `reviewed-knowledge:d8262c9800714cb23e928ecf`; exactly 3 reviewed/publication-eligible units; all other units remain generated.
- `publish-reviewed` is implemented without a new migration. It derives the unit set from the reviewed artifact and cross-checks DB content/review/evidence/provenance identities inside the publication transaction.
- Published hashing Thought Forest retrieval now uses a generic section-local lexical rerank after the existing deny-first relevance gate; no Pain-specific routing rule was added.
- Fresh isolated PostgreSQL 18 project `bodysense-s8latest` started from `sources=0/publications=0/units=0`, migrated `51 → 50 → 51`, ingested the new snapshot, and observed exactly 3 reviewed / 0 published before the explicit publication action.
- Artifact-bound dev publication exposed exactly the 3-unit cohort. The cohort published-only regression passed `9/9`: 6 exact positive hits + 3 negative expected-empty controls; publication leakage = 0.
- Imported predeploy observations produced `gate.action=continue` with 9 samples and zero retrieval/citation/grounding/identity/provenance blockers. The empty `runtime_answer` window correctly produced `hold` and binary exit code 2.
- Reingestion with `--overwrite-source` while published was rejected before writes; all 11 source ids remained unchanged.
- Rollback restored all 3 units to reviewed/unpublished, preserved 9 historical predeploy observations, and default published retrieval returned 0. A structurally valid reviewed artifact with a drifted content hash was rejected and created no publication row.
- Final local release gate: `bash scripts/validate-repo.sh` → `REPO_QUALITY=PASS`; AI-service 353/353 tests, Web 144/144 tests, Go all packages, Pyright 0 errors, migration immutability/evals/gateway smoke/build all passed.
- A final hardened real-DB v2 publication used the same reviewed snapshot, obtained batch version 2, passed the same 9/9 cohort regression and `predeploy_eval=continue`, then rolled back to exactly 3 reviewed/unpublished units.
- Thought Forest source PR #11 merged normally at `b81ac7933922c9c45016eeff9a91a359b4b82c15`. Its GitHub-hosted `Knowledge Base Gate` did not execute any steps because GitHub reported account payment/spending-limit restrictions; source CI is therefore not claimed as passed. The exact `8dbe766` head passed exporter tests, 3374-frontmatter validation, changed-only audit and drift checks locally before merge.
- Production publication has not been executed.


## Outcome

Stage 8 is complete for the bounded development/predeploy scope. The single Pain pilot has become a three-claim reviewed cohort with exact IASP evidence edges, immutable review artifacts, artifact-bound Go publication, cohort-aware hashing reranking, published-only regression, observation gates, atomic overwrite protection and rollback. Production rollout remains an explicit release operation after the merged code is deployed; absence of real `runtime_answer` samples keeps the runtime gate on `hold`.
