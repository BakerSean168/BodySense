# Published Knowledge Retrieval / Grounding Governance — 2026-08-23

## Outcome

Close the first production-like feedback loop for published Thought Forest knowledge:

```text
published unit
→ default retrieval
→ structured citation with publication identity
→ published retrieval/citation/grounding eval
→ publication observation
→ continue / hold / rollback recommendation
```

## Protected contracts

- Default RAG continues to expose only explicitly published + reviewed knowledge.
- `include_unpublished=True` remains an internal/eval-only escape hatch.
- Existing `source.citation.added` is extended additively; existing title/body fields remain.
- Diagnosis/Treatment Evidence identity and Treatment Grounding Eval v2 remain compatible.
- Go remains the authority for publication lifecycle and rollback; Python evals cannot publish or rollback DB state.
- No automatic rollback from an eval process in this stage. The gate emits a deterministic operator recommendation.

## S6-010 — Preserve publication identity through Consultation citations

**Goal:** Every citation emitted from a published Thought Forest result carries stable source/unit/publication/provenance metadata.

**Acceptance:** citation tests prove `source_key`, `unit_key`, `publication_id`, `published_version`, `source_locator`, `claim_id`, and claim review identity survive into `source.citation.added` without breaking legacy video citations.

## S6-020 — Published retrieval regression with negative relevance cases

**Goal:** Evaluate only default published retrieval and reject unrelated published results.

**Acceptance:** a versioned dataset contains positive and negative queries; positive cases retrieve the exact published unit/version and negative cases return no citation-worthy result. Threshold/gating is derived from measured pilot behavior, not guessed.

## S6-030 — Publication observations and deny-first gate

**Goal:** Attribute retrieval/citation/grounding health to one immutable publication.

**Acceptance:** PostgreSQL stores idempotent publication observations; summary reports retrieval misses, citation/provenance failures, grounding failures, identity mismatches and errors. Gate semantics:
- publication/version/provenance identity mismatch → `rollback`;
- invalid/wrong citation or rejected grounding → `rollback`;
- retrieval miss / degraded grounding / execution error → `hold`;
- clean qualified window → `continue`.

## S6-040 — Real PostgreSQL vertical verification

**Goal:** Re-run one reviewed/published Pain definition pilot through the full loop.

**Acceptance:** reviewed is invisible; publish makes only the approved unit visible; positive/negative published eval passes; observation is attributed to the exact publication; gate result is deterministic; rollback restores invisibility; migration down/up replay and full repository release gate pass.

## Non-goals

- No automatic web research or automatic claim review.
- No UI/admin console for publication governance.
- No broad publication of the remaining 114 Thought Forest sections.
- No replacement of the Treatment-specific Grounding Eval v2.
- No production deployment in this ticket; validation is on isolated GCP-dev PostgreSQL and protected CI.

## Implementation checkpoint

- `source.citation.added` now preserves stable Thought Forest unit/source/publication/claim review/Markdown locator identity; Go rejects incomplete published Thought Forest citations while legacy video citations remain compatible.
- Published visibility remains hard-gated by explicit `published` lifecycle + publication id/version + reviewed status + quality threshold.
- Hashing retrieval now requires a meaningful lexical anchor for published Thought Forest results. This was chosen from measured evidence: unrelated `PostgreSQL 索引` scored `0.3726`, while `什么是疼痛` scored `0.0515`, so a fixed cosine threshold would be unsafe.
- `bodysense.published-knowledge-eval.v1` has 6 pilot cases: 3 positive exact-unit/citation/grounding cases + 3 negative no-result controls. Real GCP-dev PostgreSQL result: `6/6 PASS`.
- migration 51 adds immutable `knowledge_publication_observations`; Python eval report is imported through Go `knowledge-publication-manager observe-eval`.
- The real publication v3 observation window stored 6 observations and returned `gate.action=continue`. A separate failure simulation with an unrelated published result returned `gate.action=rollback`.
- `observation_kind` is generic, but automatic production runtime grounding observation is deliberately not claimed complete: the current Consultation answer contract does not yet bind every generated answer claim to evidence ids. Stage 6 closes the predeploy published-governance vertical slice without fabricating that attribution.

## Validation so far

- AI-service Ruff: PASS
- AI-service Pyright: 0 errors
- AI-service unit suite: 336 passed
- Go `go test ./...`: PASS
- migration 51 live replay: `51 → 50 → 51` PASS
- published pilot eval: 6/6 PASS
- clean observation gate: `continue`
- failure simulation gate: `rollback`

Final local closeout evidence:

- full `bash scripts/validate-repo.sh`: `REPO_QUALITY=PASS`;
- migration sequence/immutability latest = 51: PASS;
- final dev publication rollback restored the pilot unit to reviewed/unpublished and default Thought Forest retrieval to 0;
- clean 6-observation audit history remains attached to the rolled-back publication.

This Stage 6 predeploy published-governance vertical slice is complete and archived. Automatic production runtime claim-to-evidence observation remains a future contract, not an unfinished item in this bounded plan.
