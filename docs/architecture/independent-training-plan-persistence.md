# Independent TrainingPlan persistence · TP-002 slice 1

Business authority: [TrainingPlan](../product/domain/training.md) and
[Safety](../product/domain/safety.md). Implements the persistence/decision slice
of [#311](https://github.com/BakerSean168/BodySense/issues/311).

`service.NewIndependentTrainingPlanService(db)` is the callable application
boundary. `Propose`, `Accept`, `Reject`, `List`, `Get`, and `Revision` require the
authenticated owner supplied by the trusted caller. `Accept` and `Reject` are
explicit manual user commands; an Agent generation result may call `Propose`
only. The service must never be passed an Agent-invented user decision. There
HTTP adapters (slice 2, below) bind to this same service; there is no second
acceptance path.

A proposal stores a complete candidate snapshot, including stable template and
exercise entry IDs, and the exact base revision for updates. A whole-document
JSON replacement records the accepted change set; before/after snapshots are
recoverable through the parent revision. No patch is silently generated or
applied after authorization. Replacement exercises must use new entry IDs;
entry and template identities cannot move to another plan.

Each write carries an idempotency key. A manual decision additionally carries
its decision ID, presented content fingerprint, target plan, and expected base
revision (both zero for create). A receipt identifies the exact user command,
proposal, plan, and immutable accepted revision. A different payload under the
same key conflicts. An exact retry returns the original receipt/revision and
recomputes Safety from current authoritative state, rather than replaying an
old execution permission.

The application transaction locks `users FOR NO KEY UPDATE`, then the owner's
`body_states FOR UPDATE`, the same head used by native BodyState and Safety
writers, then `diagnoses FOR UPDATE`. It reads the durable proposal, evaluates `EvaluateAcceptance`,
projects TrainerGate from the native ledger, writes a revision and plan CAS,
and commits the receipt and proposal decision together. Database uniqueness
and composite owner/plan FKs enforce idempotency and identity. Any write or
commit failure rolls back the plan, revision, links, and receipt. Rejected
proposals never write a plan. Expired proposals persist only their expiry
reason, with no accepted revision or decision receipt.

Context freshness is deliberately conservative in this first slice: any native
BodyState revision, Safety source availability/validity boundary, or change to
the owner's native `diagnoses.current_revision` invalidates a pending proposal.
The transaction locks the native Diagnosis head after the BodyState head, so
concurrent adoption cannot race its snapshot. A missing aggregate is initialized
at revision zero; it creates no ConditionEntry or DiagnosisRevision and does not
prevent an independent plan with no Diagnosis association. Legacy
`diagnosis_analyses` are never read as an acceptance authority.

Related ConditionEntries must belong to the owner, be adopted, and refer to a
committed revision within the locked head. Their current snapshots and typed
evidence links enter the fingerprint. Current source availability is re-read
from owner-bound statement evidence, formal analysis request evidence, and
admitted document sources; the original assessment-time claim validity is not
an availability authority. Shared source locks keep the checked source stable
through commit. Missing, blocked, unresolvable, or unreadable associated sources
fail closed. These checks preserve existing Diagnosis semantics; they neither
adopt diagnoses nor infer clinical eligibility from source text. Reads are
bounded to 200 related conditions and 1,000 evidence links. This is not a
clinical relevance algorithm. No Diagnosis lifetime or clinical threshold is
introduced. The Training PlanProposal TTL alone is 168 hours.

Training §4.5 permits saving a prescription under an existing restriction.
Consequently the returned TrainerGate is a current **execution projection**,
not a condition that can change plan status or authorize training from a save
receipt. Missing/unscreened/unreadable Safety never projects allow. A stale
proposal context expires before any plan write. A Trainer start/continue must
always perform its own fresh gate check; Trainer sessions/actions are TP-003.

BodyFact, SymptomEpisode, and BodyObservation links validate exact owner and
creation revision and use composite FKs in revision links. Native ConditionEntry
associations use `training_plan_revision_conditions` with owner composite FKs to
both the immutable plan revision and the native condition. Foreign/missing
related IDs produce the same owner-masked not-found result at proposal creation
and acceptance; a known association with unreadable authority is unavailable.

The baseline drops the single-active index. Existing Treatment columns and
consumers remain until slice 3 retires them; no compatibility adapter or flag
is added. Their existing supersede query cannot touch a plan with an accepted
independent revision. Independent plan updates preserve the user's existing
active/paused/completed status and never touch another plan or any logs.

Validation: `scripts/plan/test-tp002-pg18.sh` creates only its own disposable,
network-isolated, memory-bounded PostgreSQL 18 container and Unix socket. It
validates baseline up/down/up and runs the service acceptance tests. Add
`-race` for the serialized race run. The database CI lane runs the same native
service regressions with `-race`. `TP_D1_EXPECTATION=v2
scripts/plan/test-tp-d1-multi-active-compat.sh` proves the removed unique index.
No staging/production access or retained database reset is part of this work.

## Slice 2a · HTTP surface and user-managed lifecycle

Public routes (OpenAPI `bodysense.v1`), all owner-scoped by the authenticated
user; a foreign or missing id is the same 404:

| Route                                                           | Meaning                                                                                                                               |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /api/v1/training-plans`                                    | every accepted plan in any status (several may be active)                                                                             |
| `GET /api/v1/training-plans/{planId}`                           | one plan: status, `statusRevision`, current content, `review`, `executionGate`                                                        |
| `GET /api/v1/training-plans/{planId}/revisions/{revisionId}`    | one immutable accepted revision                                                                                                       |
| `POST /api/v1/training-plans/{planId}/status`                   | explicit user status change: `status`, `expectedStatusRevision`, `idempotencyKey`                                                     |
| `GET/POST /api/v1/plan-proposals`                               | list (optional `state`) / save the user's own draft as a pending proposal                                                             |
| `GET /api/v1/plan-proposals/{proposalId}`                       | one proposal (pending past 7 days reads as expired)                                                                                   |
| `POST /api/v1/plan-proposals/{proposalId}/accept` and `/reject` | the user's explicit decision: `decisionId`, `presentedFingerprint`, `targetPlanId`/`expectedRevisionId` for updates, `idempotencyKey` |

`POST /plan-proposals` always records `initiatedBy=user`; Agent generation
calls `Propose` in Go and never this route.

**Status is user-managed** (training.md §4.2). `ChangeStatus` takes the owner
lock (`users FOR NO KEY UPDATE`), checks `expectedStatusRevision`, applies the
pure `trainingplan.StatusTransitionAllowed` table (active↔paused,
active/paused→completed, completed→active on the same planId), compare-and-sets
`training_plans.status/status_revision`, and appends one immutable
`training_plan_status_events` row (DB CHECK repeats the transition table;
`UNIQUE(user_id, plan_id, status_revision)` and `UNIQUE(user_id,
idempotency_key)`). It never touches content, revisions, logs or other plans,
and never grants execution.

**Nothing else writes a plan.** There is no `review_recommended` status:
`review` is computed on read by comparing the accepted revision's proposal
context fingerprint with the current one (same conservative fingerprint as
acceptance), so any BodyState revision, Safety boundary or native Diagnosis
change recommends review; an unreadable context reports
`context_unavailable`. With the context unchanged, user feedback on a Trainer
session of the current accepted revision recommends review as
`session_feedback_since_acceptance`; `review.sessionFeedbackCount` reports how
many such sessions there are (`docs/architecture/trainer-runtime.md`). The Safety pause is `executionGate`, the Go TrainerGate
evaluated on every read: `permitsExecution=false` for block/unavailable, and
missing, unscreened or unrecognized Safety is never allow. Neither projection
modifies status, content or revisions.

## Slice 2b · Treatment generation → agent `PlanProposal(create)`

`POST /api/v1/plan-proposals/generations` (`generatePlanProposal`) takes the
authenticated user's generation command: optional `diagnosisAnalysisId`
(latest when omitted), `userConstraints`, `changeReason` and an
`idempotencyKey`. The owner comes only from auth; the body cannot carry an
owner, draft, initiator or decision. It returns the pending agent proposal as
`PlanProposalRecord`: 201 when new, 200 for an idempotent replay.

`TreatmentService.GeneratePlanProposal` reuses the Treatment generation core
(`runGeneration`): the Go decision policy and Safety gate, the frozen replay
input, the Agent call, and the configuration identity, governance and
evidence-trace checks. Python projects the governed output onto a strict
`training-plan-agent-proposal-v1` block:

```json
{
  "schemaRevision": "training-plan-agent-proposal-v1",
  "outcome": "proposed | unavailable",
  "reason": "…(unavailable only)",
  "title": "…",
  "goal": "…",
  "summary": "…",
  "content": {
    "sessions": [
      {
        "title": "…",
        "stages": [
          {
            "phase": "warmup|main|cooldown",
            "exercises": [
              {
                "exerciseId": "chin_tuck",
                "nameSnapshot": "…",
                "sets": 2,
                "target": {
                  "kind": "repetitions|duration|distance",
                  "min|max|seconds|meters": 1
                },
                "restSeconds": 30,
                "side": "…",
                "loadOrIntensity": "…",
                "instructions": "…"
              }
            ]
          }
        ]
      }
    ],
    "frequency": { "sessionsPerWeek": 3, "description": "…" },
    "constraints": []
  },
  "evidenceIds": []
}
```

- Go decodes it with unknown fields rejected, so an Agent-chosen template,
  entry, plan, revision or owner ID, a schedule, or a decision fails the whole
  output. Go assigns every `templateId`/`planExerciseId`, sets
  `schedule.mode=frequency_only` and runs the same `validatePlanDraft` as user
  drafts. Evidence IDs must be ones the governed output cited.
- `unavailable` (governance rejected or no plan) and any invalid block persist
  nothing (HTTP 502). The Go Diagnosis/Safety gates refuse before the Agent
  runs (HTTP 409).
- The result is one `PlanProposal(kind=create, initiatedBy=agent,
sourceId=treatment_agent:<configurationId>)`. No Treatment revision is
  written, and no TrainingPlan is created, changed or activated. Only the
  user's explicit accept writes a plan.
- `plan_proposals.replay_input` holds the exact frozen Agent input and
  `generation_provenance` the Agent configuration, execution, governance,
  evidence trace, Go decision trace, rollout route and the digests of the
  replay input and Agent block. Both columns are present together, only on
  agent proposals. A trigger keeps every proposal's candidate, fingerprints and
  generation audit immutable; only `state`, `decision_id` and
  `expired_reason` change.
- The idempotency key is bound to the user's command (owner, analysis,
  constraints, change reason), not to the Agent output. A retry with the same
  command returns the stored proposal without calling the Agent; a different
  command under the same key is a 409. If the BodyState revision changed while
  the Agent ran, the proposal is stale: 409 with no write.

Slice 3 removed the legacy `POST /treatments/proposals` path together with
Treatment (see below).

## Slice 3 · Treatment removed

TrainingPlan + PlanProposal is now the only plan authority. The Treatment
aggregate, its lifecycle and every link to it are gone:

- **HTTP.** The `/treatments/*` routes are removed. `HealthWorkspace` drops
  `treatment`, `treatment_revisions`, `can_accept_treatment`,
  `can_review_treatment` and `requires_treatment_review`.
  `can_generate_treatment` keeps its Diagnosis + Safety gate (generation yields
  a pending PlanProposal); `can_execute_treatment` and `can_record_outcome`
  follow the active TrainingPlan, and execution still needs complete Safety
  coverage.
- **Outcomes** stay the user's longitudinal feedback in their own
  `OutcomeService`/`OutcomeRepository`. `Outcome` loses `treatment_id`,
  `treatment_revision_id` and `intervention_id`; a user-authored symptom still
  commits a BodyState fact in the same transaction.
- **Legacy `/training/*`** stays until TP-003 but no longer creates,
  supersedes or follows a plan from a Treatment:
  `AcceptTreatmentAndEnsurePlan`, `EnsurePlanForTreatment`,
  `EnsureCurrentPlan`, the reassess-driven Treatment regeneration,
  `SupersedePlansExcept` and `GetPlanByTreatmentRevision` are deleted, and the
  contract loses its Treatment and intervention fields. Plans are created
  only by accepting a PlanProposal. The TP-D1 opt-in owner-lock path in the
  legacy service went with them; the independent TrainingPlan service keeps
  its own owner lock.
- **Generation.** `TreatmentService` keeps only the Treatment Agent generation
  core (`runGeneration`, `GeneratePlanProposal`). The Go decision policy keeps
  its `treatment-go-acceptance-v1` identity, which Agent configurations pin,
  but only its generation phase. The deployment route (champion, shadow,
  canary) is still recorded in `generation_provenance.rolloutProvenance`;
  the Treatment replay service, the rollout-observation service and
  `cmd/treatment-rollout-status` are deleted, so no shadow run is executed or
  observed.
- **Schema.** The baseline migration is edited in place. Dropped tables:
  `treatments`, `treatment_revisions`, `interventions`,
  `treatment_rollout_observations`. Dropped columns with their foreign keys:
  `training_plans.treatment_id`, `training_plans.treatment_revision_id`,
  `training_logs.treatment_revision_id`, `training_logs.intervention_id`,
  `outcomes.treatment_id`, `outcomes.treatment_revision_id`,
  `outcomes.intervention_id`. Dropped indexes:
  `idx_training_plans_treatment_revision`, `idx_outcomes_treatment`,
  `idx_training_logs_intervention`. `checksums.sha256` and
  `docs/refactor/vnext/schema/database-schema.json` are regenerated; the
  frozen Phase 00 snapshot under `baseline/` is not.
- **Privacy erasure** needs no privileged pre-delete any more: the only
  `ON DELETE RESTRICT` lineage (TreatmentRevision → DiagnosisAnalysis) is
  gone, so every user-owned row cascades from the user.
- **Web.** The Treatment panel and the Treatment-backed proposal section are
  removed; UI-TP-001 rebinds proposals to `/plan-proposals`. Longitudinal E2E
  creates plans through `/plan-proposals/generations` + `/accept`.
- **Validation.** `scripts/local-deploy-validate.sh` checks Treatment Agent
  output as `plan_proposals` rows (`source_id=treatment_agent:<config>`) with
  a generation decision trace and replay input.
  `TP002_FULL_CHECKS=1 scripts/plan/test-tp002-pg18.sh` runs the migration and
  domain validators, `go vet` and the race test suite against a disposable
  PG18.
