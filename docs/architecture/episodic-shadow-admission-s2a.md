# Episodic S2-A — isolated, default-off Go shadow draft admission

Parent: #498 · generator ticket: #515 · prerequisite source-erasure inventory: #514.

## Delivery boundary (2026-11-05)

S2-A provides **only a repository-level admission mechanism**. No production
constructor or application wire-up calls `WithShadowAdmissionEnabled(true)`.
There is no Python invocation, scheduled job, HTTP endpoint, UI, retrieval,
clinical input or remote model payload. The database still constrains every
`episodic_notes.status` to `shadow`. This PR must not close #515 or authorize
Staging/Production usage.

Before any draft is accepted, the Go transaction locks the owner singleton
Conversation using the same row lock as raw-message prune/turn admission, and
proves all of these properties:

- The `from_seq..to_seq` range is positive, contiguous and has an **exactly
  matching, ordered and deduplicated list of 1–128 message UUIDs**.
- Source messages are still physically present, text-only (`parts=[]`),
  nonempty, completed, owned by the same Conversation, associated with complete
  turns and with completed Runs (if one is attached).
- The processed cursor `P` has passed the range and the raw context start
  `C` moved past it, but prune cursor `R` has not yet erased its beginning.
  A normal prune is never delayed by any note or its generation failure.
- None of the referenced messages is linked to `statement_evidence`,
  `safety_screens` or `safety_resolutions`. Any asset/multimodal reference,
  any future `SourceAssetIDs` in the draft and any partial turn fail closed.
  This is **not** a guarantee that arbitrary natural language is medically
  neutral: full semantic non-clinical verification remains an S2-B gate.
- The draft is UTF-8, 1–800 runes, uses bounded generator and retention policy
  IDs, and is written atomically as a non-serving `shadow` note. Repeated
  range+version+policy writes replay the exact existing note; changed content
  or source IDs are rejected as conflicts.
- A provisional **test-only** owner cap bounds retained rows (48) and total
  characters (16,000) through oldest-first rollover. These values are *not*
  an approved product privacy retention policy; the policy owner and a
  12-month synthetic-bound benchmark must approve the eventual limits.

## Source deletion and owner boundaries

S3-A #518 and S3-B #519 already added statement/asset copy erasure targets,
finalize rechecks, and post-deletion database admission triggers. S2-A accepts
no asset-bearing source and rejects any existing clinical statement source.
`source_message_ids` remains soft provenance without an FK to `messages`:
a successful prune can physically delete messages without blocking.

Only artificial fixtures exercise enabled admission. The isolated PostgreSQL
18 tests cover owner isolation, complete ordered provenance, clinical
exclusion, partial-turn prevention, stale/streaming/multimodal exclusion,
cross-transaction Conversation row locking, retries after prune, rolling cap
and unchanged S3 erasure. Tests do **not** use patient data.

## Remaining S2-B and Stage 2 gates

- S2-B1 adds a narrow **offline, default-off typed Go↔Python quote proposal**
  contract for four exact non-clinical response-style preferences. See
  [S2-B1 quote proposal](episodic-shadow-s2b-quote-proposal.md). This is not
  approval of a live model provider, arbitrary narrative generation or
  private health-data egress; source IDs and final canonical text remain
  Go-owned and Go-validated.
- Prove 100% source coverage for **all** narrative/asset source content and
  detect indirect clinical fact restatements. Add frozen ≥100-note trace audit,
  ≥98% claim traceability, zero untraceable clinical claims and zero clinical
  authority leakage.
- Wire shadow generation to a safe, best-effort post-context-boundary
  hook only after policy approval, without slowing or aborting prune. Keep
  kill switches for generation and serving independent.
- User list/inspect/delete controls (S6, #516), manual deletion anti-recreation
  policy, note serving/recall controls (S4, #517) and separate clinical
  authority verification remain required. Never enable serving from this PR.

The existing ADR `episodic-continuity-decision.md` remains authoritative
for source/clinical, product and rollout decisions.
