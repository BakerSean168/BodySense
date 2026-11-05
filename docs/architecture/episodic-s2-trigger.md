# Episodic S2 — best-effort shadow generation at the prune boundary

Parent: #498 · generator ticket: #515 · prerequisites: S2-A admission (#521),
S2-B1 typed quote proposal (#522), S3 erasure (#514), S6 deletion tombstones
(#516). ADR: [episodic-continuity-decision.md](./episodic-continuity-decision.md),
rule 2.

## Delivery boundary (2026-11-05)

This slice **connects the already-merged narrow path** to the chat-retention
prune boundary. It adds no new model capability:

- A segment is exactly one complete two-message interaction (`user` +
  `assistant`). It must be contiguous, settled, text-only (`parts = []`),
  non-empty, unpadded, within the S2-B1 size bounds, free of
  `statement_evidence`/`safety_screens`/`safety_resolutions` references, free
  of a Diagnosis analysis/direct-edit request reference, and free of a
  user-deletion tombstone. Anything else is recorded as `skipped` and never
  sent to a model.
- The proposal remains the S2-B1 typed command: exact whole-user-message
  quotes from the four approved non-clinical reply-style preferences. Go
  constructs the stored canonical text. There is no free-form summary, no
  clinical vocabulary, no note retrieval and no clinical input.
- Up to two messages immediately preceding the segment may be included as
  **untrusted context** (`context_messages`, max 2). Context is never part of
  the note's provenance, never quotable and never stored. The note's
  `from_seq..to_seq` and `source_message_ids` cover the new segment only.
- Generation is best-effort. The trigger runs after `PlanPrune` and before
  checkpoint cleanup/commit, so the source rows still exist. It returns no
  error that can abort the pass; checkpoint cleanup, deletion and `R` movement
  continue on schedule.
- No note is served. `episodic_notes.status` remains `shadow`. Production
  constructs the trigger with **no proposer**, so it is inert even if the flag
  is set. There is no HTTP route, worker, scheduler, UI action, real model
  provider or data egress in this slice.
- No BodyState, Diagnosis, Safety, TrainingPlan or serving path is touched.

## Where the trigger runs

```
PlanPrune (Conversation lock, plan)
        │
        ├─ Best-effort distillation (this slice)
        │     segments in (D, plan.TargetSeq]
        │     RecordPlan → bounded proposals → SaveShadowDraft
        │     advance D over the resolved prefix only
        │
        ├─ ai-service checkpoint cleanup (unchanged)
        ├─ CommitPrune deletes raw rows, advances R (unchanged)
        └─ AfterPrune: pending rows inside R become recorded gaps; D = R
```

`D` is `conversations.distilled_through_seq`. It is not a retention cursor:
it never moves on its own, never blocks pruning, does not bump
`retention_revision` (so a plan taken before the trigger stays valid) and is
constrained to `0 <= D <= P`. `R`, `C` and `P` keep their existing meaning and
ordering.

## Segment ledger and watermark

Migration `000007_episodic_distillation` adds:

- `conversations.distilled_through_seq integer not null default 0`.
- `episodic_distillation_segments (user_id, conversation_id, from_seq,
  to_seq, status, attempts, note_id, reason, timestamps)`.

The ledger is content-free: no chat text, no message UUIDs, no model output,
no clinical detail. Statuses are `pending`, `distilled`, `skipped` and
`gapped`. A row cascades with the owning Conversation; `note_id` is set to
NULL if the note is later deleted or rolled off.

`RecordPlan` writes a durable row for every enumerated segment **before** any
model call, in one transaction. A crash therefore cannot leave a later
segment with no record at all. `D` advances only over the contiguous resolved
prefix: one unresolved `pending` segment stops the watermark for that pass,
even if later segments resolved.

After a prune commits, `HealAfterPrune`:

1. marks every `pending` row with `to_seq <= R` as `gapped`
   (`pruned_before_distillation`), because its source is gone by design;
2. if the trigger never enumerated the window at all, writes one coarse
   content-free gap row (`pruned_unenumerated`) rather than losing the fact;
3. sets `D = R`.

`AfterPrune` uses a detached context so a cancelled job still records the gap.
A later pass that starts below `R` (crash) heals the same way before doing
anything else.

## Retries, bounds and gap reasons

| Bound | Value | Effect |
| --- | --- | --- |
| Attempts per segment | 3 | After the third failure the segment becomes `gapped` and is released. |
| Proposals per prune pass | 8 | Remaining eligible segments are recorded as `pass_budget` gaps. |
| Per-proposal timeout | 5 s | A timeout is a retryable `timeout` failure. |
| Whole-trigger budget | 20 s | Stops new model calls; unrecorded work becomes a pruned gap. |

Failure reasons are stable codes (`timeout`, `model_error`,
`source_invalid`, `conflict`, `user_deleted`, `retry_limit`, `pass_budget`,
`pruned_before_distillation`, `pruned_unenumerated`), not raw provider text.
Every gap is logged and counted; the prune job also receives an
`episodic.distillation` event with content-free counters (`considered`,
`distilled`, `abstained`, `skipped`, `gaps`, `failures`, `retries`). A failed
model call never prevents deletion.

## Idempotency and anti-regeneration

- Repeating a segment at the same generator/policy version returns the exact
  existing note (`episodic_notes_idempotency_key`); changed content or source
  IDs are a conflict (`ErrEpisodicDraftConflict`).
- The durable ledger row is terminal (`distilled`/`skipped`/`gapped`), so a
  replay after a crash between note write and watermark update does not call
  the model again or create a second row.
- S6-A tombstones are checked twice: the segment pre-filter marks an
  overlapping range `user_deleted` and does not call the model at all, and
  `SaveShadowDraft` rechecks under the Conversation lock before any insert.
  A deleted note's range cannot be regenerated by a retry, a new generator
  version or an expanded window.
- A concurrent user deletion can still race a model call that already left
  Go; the tombstone then blocks the write, and later passes never resend the
  range. The note is never persisted.

## Switch and wiring

`EPISODIC_SHADOW_GENERATION_ENABLED` follows the existing S6 switch shape:
only `true` or `1` opts in; unset/every other value is off. The production
constructor is wired with a nil proposer, so the env flag alone logs a warning
and leaves generation off. Enabling a real provider additionally requires an
approved proposer, an admission-enabled writer, and the Stage 1 privacy/egress
and benchmark decisions. Tests inject a fake proposer and never contact a
provider.

With the trigger absent or disabled, the prune path is unchanged: no ledger
row, no watermark movement, no model call, no extra job event. The
`FlagOffPreservesPrune` test compares a disabled run against a control run.

## Verification evidence

Local, disposable PostgreSQL 18, synthetic data only:

- `FULL_UP=PASS version=7`, `LATEST_DOWN=PASS version=6`,
  `LATEST_REPLAY_UP=PASS version=7` via `cmd/migration-validator`.
- `scripts/quality/run-go-db-tests.py --profile pr`: 26/26 PASS with the race
  detector.
- Repository: watermark/ledger, retry ceiling, coarse gap, clinical/
  multimodal/partial rejection, tombstone context suppression and rollback
  refusal with an unresolved gap.
- Service: prune-boundary note and provenance, bounded lookback, ineligible
  segments without a model call, unsettled-Run deferral, failure without
  prune blocking, three-attempt retry, replay idempotency, tombstone
  anti-regeneration/egress, and flag-off parity.
- Go vet/build and contract generation checks remain mandatory in CI.

PR CI is the authority for the full suite; this slice deliberately does not
run the full repository or Playwright suites locally.

## Not authorized by this slice

- No real model provider, data-egress/consent approval or production model
  credentials.
- No `n>=100` live-model claim traceability/clinical-authority audit and no
  clinical/safety regression benchmark; the 120 synthetic cases remain
  structural only.
- No note serving/recall (S4 #517), no user memory UI (#516), no new vector
  database and no change to the retention window or privacy policy.
- No Staging/Production deployment, migration execution or Supabase change.

The ADR remains authoritative for source/clinical, product and rollout
decisions.
