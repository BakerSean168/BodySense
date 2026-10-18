# Trainer runtime (TP-003)

Status: slices 1–3a of [#313](https://github.com/BakerSean168/BodySense/issues/313).
Product rules: `docs/product/domain/training.md` §4.3–4.4 (TP-D6..D10) and
`docs/product/domain/safety.md`. Executable reference model:
`scripts/plan/v2-trainer-actions-model.test.mjs`.

## Model

- **TrainingSession** (`training_sessions`) is one execution instance. It pins
  the plan's current accepted revision and one template, the user's IANA
  timezone snapshot and `training_date`, the local date at start. The date
  never changes, including when the session crosses local midnight; two
  sessions on one day are two rows. State is `in_progress ↔ paused → ended`.
  A user has at most one `in_progress` session (partial unique index);
  paused sessions do not hold that slot. The start receipt (idempotency key,
  request fingerprint, Safety revision, gate result and policy revision) is
  immutable; only `state` and `revision` change, and the single transition to
  `ended` (from `in_progress` or `paused`) records `finished` or
  `stopped_early` with its own idempotency key.
- **TrainingSessionTransition** (`training_session_transitions`) is the
  immutable receipt of one explicit `pause` or `resume`: session revision,
  idempotency key and, for a resume only, the Safety revision and gate result
  that admitted it. Paused time is derivable from these receipts.
- **TrainingSetAction** (`training_set_actions`) is an immutable receipt of
  one explicit user action on a target of the pinned template:
  - `scope=set`: one set `performed` (optionally `actual_reps` /
    `actual_duration_seconds`, which may differ from the prescription) or
    `skipped`;
  - `scope=exercise`: the rest of a whole exercise `skipped` as one receipt,
    never as fabricated per-set skips;
  - `corrected`: retracts `corrects_action_id`, an effective receipt of
    exactly the same target in the same session, at most once. The original
    stays; the target may then be recorded again.

  A receipt is **effective** unless it is a correction or a correction points
  at it; a set, and an exercise skip, hold at most one effective receipt, and
  a set cannot be recorded under an effective exercise skip. Every receipt
  carries the session revision it created, its idempotency key and the exact
  Safety revision and gate result that admitted it. Composite owner FKs keep
  receipts on the owner's own session.

## Commands (Go only)

| Route                                                            | Command                                                                | Gate                                          |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------- | --------------------------------------------- |
| `POST /training-sessions`                                        | start a template of an owned `active` plan at `expectedPlanRevisionId` | `start_session` over the template's exercises |
| `POST /training-sessions/{id}/set-actions`                       | `performed` / `skipped` / `corrected` at `expectedSessionRevision`     | `perform_set` for that exercise               |
| `POST /training-sessions/{id}/pause`                             | `in_progress → paused` at `expectedSessionRevision`                    | none: pausing is always allowed               |
| `POST /training-sessions/{id}/resume`                            | `paused → in_progress`; plan still `active`, no other in progress      | `resume_session` over the template            |
| `POST /training-sessions/{id}/finish`                            | `finished` / `stopped_early` at `expectedSessionRevision`              | none: stopping is always allowed              |
| `POST /training-sessions/{id}/feedback`                          | the user's verbatim words on an `ended` session, once                  | none: feedback is always allowed              |
| `GET /training-sessions[?state=]`, `GET /training-sessions/{id}` | read receipts                                                          | none                                          |

Each command runs in one transaction under the owner lock, then:

1. An idempotency key already used by this owner replays the stored result if
   the request fingerprint matches, otherwise `REVISION_CONFLICT`.
2. Gated commands share-lock the BodyState head and require its revision to
   equal `expectedSafetyRevision`, the revision of the gate the user was shown
   (`executionGate.safetyRevision` on the plan record). Any other value is
   `SAFETY_STALE`. Then the Go TrainerGate must permit (`allow`/`warn`);
   `block` or `unavailable` is `TRAINER_GATE_BLOCKED`. Missing or unscreened
   Safety never permits.
3. Only then is the receipt written. A refusal writes nothing and never
   advances the session.

Foreign plan or session ids are the same owner-masked 404 as missing ones.
The runtime only reads plans; it never changes plan status, content or
revisions, and nothing is inferred from navigation, playback or Agent output.

A paused session accepts no set action (`SESSION_PAUSED`); resuming a
session that is not paused is `SESSION_NOT_PAUSED`. Corrections are gated
like any set action, so a correction also needs the current Safety revision
and a permitting gate.

## Session feedback (TP-D10)

`training_session_feedback` holds the user's own words about one ended
session, verbatim (1–2000 characters, not only whitespace, never trimmed or
rewritten), at most one row per session, with an idempotency key and request
fingerprint. It pins the session's plan and revision through a composite FK;
a DB trigger refuses rows for sessions that have not ended and rows are
immutable. `SESSION_NOT_ENDED` and `FEEDBACK_ALREADY_RECORDED` are visible
409s. Feedback is never gated: the user can always say how a session went.

Feedback is **never interpreted**. It writes no plan, revision, status,
receipt, BodyState or Safety row and is not turned into a symptom or a
clinical fact; a body change the user reports is recorded through BodyState
(chat intake or `/body-state` commands). Its only effect is the read-only
plan review projection: `TrainingPlanRecord.review.sessionFeedbackCount`
counts sessions of the **current accepted revision** with feedback, and when
the context is otherwise unchanged `review` is
`recommended=true, reason=session_feedback_since_acceptance`. A context
change or unreadable context keeps precedence. Accepting a new revision
resets the hint (feedback stays on the sessions of the old revision). Only a
user-accepted PlanProposal changes the prescription.

## Not yet

- deleting legacy `/training/*` and `training_logs` (slice 3b);
- Safety-driven hold/stop transitions (`safety_hold`, `safety_stopped`) of
  the reference model; today a non-permitting gate refuses the command and
  the user may stop;
- Agent trainer tools and the web Trainer rebinding (UI-TP-001).
