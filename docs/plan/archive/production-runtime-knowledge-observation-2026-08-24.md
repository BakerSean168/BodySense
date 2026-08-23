# Production Runtime Knowledge Observation — 2026-08-24

## Outcome

Close the first real Consultation runtime attribution loop for published knowledge:

```text
published search result
→ stable evidence_ref exposed to the model
→ explicit answer-attribution tool call
→ runtime validates attribution only against evidence retrieved in this turn
→ deterministic grounding status
→ source.answer_attribution.added
→ Go trust-boundary validation
→ durable runtime event + assistant message part
→ knowledge_publication_observations(runtime_answer)
→ continue / hold / rollback summary
```

## Current gap

Stage 6 proved published retrieval/citation/provenance before deployment, but Consultation final answers are still plain streamed text. A citation can coexist with an answer without proving which material answer claim used which evidence. Therefore production observation cannot safely equate “citation appeared” with “answer was grounded.”

## Protected contracts

- Keep natural-language Consultation replies streamed as `message.text.delta`; no whole-response JSON buffering.
- `source.citation.added` remains the source-display contract and does not become answer attribution.
- Existing Diagnosis/Treatment typed `evidence_ids` stay unchanged.
- Only evidence actually returned by `search_knowledge` in the current turn can be attributed.
- Published Thought Forest identity remains publication_id + publication_key/batch + version + unit/claim/review + Markdown locator.
- Go remains publication lifecycle and observation authority; Python cannot publish or rollback.
- Runtime observation is non-blocking for the user response. Recording failures are logged and never mutate publication state automatically.
- No migration in this stage: reuse migration 51 observation storage so this work does not collide with concurrent migrations 52–54.

## S7-010 — Stable runtime evidence reference

**Goal:** Give the model an opaque, exact reference for each published Thought Forest result without exposing DB row IDs as authority.

**Acceptance:** `search_knowledge` tool text includes an `evidence_ref` derived from immutable publication/unit identity; raw runtime state stores the exact publication binding and resets it each turn.

## S7-020 — Explicit answer attribution tool

**Goal:** Require a structured model action for material claims derived from published knowledge.

**Acceptance:** new query-only `record_answer_attribution` tool accepts bounded `claim_text + evidence_refs[]`; runtime rejects unknown/stale refs, evaluates deterministic lexical grounding against the exact retrieved reviewed claim, and emits one validated attribution event per claim.

## S7-030 — Cross-runtime event contract

**Goal:** Preserve attribution across Python → Go → durable runtime events without changing visible text streaming.

**Acceptance:** `source.answer_attribution.added` exists in shared fixtures/schema/TS parser/Python mapping/Go validation/persistence; malformed publication identity or provenance is rejected at Go AIClient trust boundary; web may ignore the event visually but must parse it safely.

## S7-040 — Automatic publication runtime observation

**Goal:** Convert validated runtime attribution into publication-version quality evidence.

**Acceptance:** Consultation Runtime records idempotent `runtime_answer` observations keyed by run/message/attribution/publication. `supported` contributes clean evidence; `degraded` causes hold; `rejected` or identity/provenance corruption causes rollback recommendation. If a published Thought Forest citation reaches a completed answer without any attribution for that publication, record a degraded `missing_answer_attribution` observation rather than silently treating it as grounded.

## S7-050 — Vertical verification and closeout

**Goal:** Prove the contract end-to-end without touching the concurrent security worktree.

**Acceptance:** focused Python/Go/contracts/web tests pass; a production-like stream fixture produces citation + attribution + durable observation; missing attribution produces hold; invalid binding is rejected; full `scripts/validate-repo.sh` passes; plan is archived and merged through protected CI.

## Non-goals

- No semantic LLM judge in the synchronous request path.
- No claim-by-claim UI visualization in this stage.
- No automatic rollback execution.
- No publication of additional Thought Forest claims.
- No attempt to retrofit free-form historical answers that lack attribution.
- No change to concurrent auth/privacy/security work.

## Implementation checkpoint

- Added current-turn `Published Evidence Ref` bindings for published Thought Forest search results; refs are reset on every Consultation turn.
- Added query-only `record_answer_attribution` tool and prompt contract. Unknown/stale/invented refs fail closed as tool errors; final user text remains ordinary `message.text.delta` streaming.
- Added conservative deterministic attribution grounding with claim-level and per-publication-binding `supported / degraded / rejected` results. This is deliberately not presented as a semantic medical judge.
- Added shared `source.answer_attribution.added` StreamEvent across schema, fixtures, TypeScript parser, Python mapping, Go AIClient validation and durable runtime-event persistence.
- Go validates publication UUID/key/batch/version, unit/claim/review identity, exact evidence-ref coverage and Markdown provenance at the AIClient trust boundary.
- Consultation Runtime waits until the assistant message is successfully completed/persisted, then converts final citation + attribution parts into idempotent `runtime_answer` publication observations.
- A published citation without attribution records `degraded / missing_answer_attribution` and therefore holds the publication. Attribution whose matching citation is absent from the final persisted message records `citation_status=invalid` and therefore recommends rollback.
- Multi-evidence claims record grounding per publication binding so one supporting source cannot mask another unsupported source.
- Reused migration 51 observation storage; no Stage 7 migration was added, avoiding collision with concurrent migrations 52–54 in the isolated security worktree.

## Validation checkpoint

- Contracts parser fixture tests: PASS (10/10).
- Contracts TypeScript typecheck: PASS.
- Web typecheck: PASS.
- Web tests: PASS (144/144).
- Go `go test ./...`: PASS.
- AI-service Ruff: PASS.
- AI-service Pyright: 0 errors.
- AI-service full unit suite: PASS (347/347).
- Focused answer-attribution/search/runtime tests: PASS.
- Full repository release gate: PASS (`REPO_QUALITY=PASS`).

## Closeout

**Status:** completed 2026-08-24.

The Stage 7 vertical slice is complete: Consultation can now explicitly bind material final-answer knowledge claims to exact current-turn published evidence, preserve that attribution across the shared stream contract, and record publication-version runtime observations only after the final assistant message is durably persisted. Missing attribution degrades/holds; attribution without the persisted citation is a blocking rollback signal. No automatic rollback or semantic-judge claim is introduced.
