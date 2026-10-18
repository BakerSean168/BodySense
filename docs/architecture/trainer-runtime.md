# Trainer runtime (TP-003)

Status: slice 1 of [#313](https://github.com/BakerSean168/BodySense/issues/313).
Product rules: `docs/product/domain/training.md` §4.3–4.4 (TP-D6..D10) and
`docs/product/domain/safety.md`. Executable reference model:
`scripts/plan/v2-trainer-actions-model.test.mjs`.

## Model

- **TrainingSession** (`training_sessions`) is one execution instance. It pins
  the plan's current accepted revision and one template, the user's IANA
  timezone snapshot and `training_date`, the local date at start. The date
  never changes, including when the session crosses local midnight; two
  sessions on one day are two rows. A user has at most one `in_progress`
  session (partial unique index). The start receipt (idempotency key, request
  fingerprint, Safety revision, gate result and policy revision) is immutable;
  only `revision` advances, and the single `in_progress → ended` transition
  records `finished` or `stopped_early` with its own idempotency key.
- **TrainingSetAction** (`training_set_actions`) is an immutable receipt for
  one set the user explicitly marked `performed` or `skipped`. It is keyed by
  `(session, plan_exercise_id, set_number)` (one action per set), carries the
  session revision it created, its idempotency key and the exact Safety
  revision and gate result that admitted it. Composite owner FKs keep a
  receipt on the owner's own session.

## Commands (Go only)

| Route                                                            | Command                                                                | Gate                                          |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------- | --------------------------------------------- |
| `POST /training-sessions`                                        | start a template of an owned `active` plan at `expectedPlanRevisionId` | `start_session` over the template's exercises |
| `POST /training-sessions/{id}/set-actions`                       | `performed` / `skipped` at `expectedSessionRevision`                   | `perform_set` for that exercise               |
| `POST /training-sessions/{id}/finish`                            | `finished` / `stopped_early` at `expectedSessionRevision`              | none: stopping is always allowed              |
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

## Not in slice 1

- pause/resume, exercise-scope skip and `corrected` actions (TP-D7);
- actual reps/duration per set and subjective feedback → review projection;
- deleting legacy `/training/*` and `training_logs`;
- Agent trainer tools and the web Trainer rebinding (UI-TP-001).
