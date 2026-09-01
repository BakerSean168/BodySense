# Feature Spec: Training Execution and Outcome Feedback

> Status: Current implemented feature contract
> Updated: 2026-09-01
> Historical schedule/adaptation design: [`docs/plan/archive/architecture-snapshots/2026-09-01/feature_spec_training_schedule.md`](./plan/archive/architecture-snapshots/2026-09-01/feature_spec_training_schedule.md)
> Domain anchor: [Longitudinal Health Loop](./architecture/longitudinal-health-loop.md)

## 1. Product role

Training is the execution projection of an **accepted TreatmentRevision**. It is not an independent AI-owned plan authority.

```text
DiagnosisAnalysis
  -> Treatment proposal/revision
  -> user/domain acceptance
  -> TrainingPlan execution projection
  -> daily TrainingLog / check-in / feedback
  -> Outcome
  -> Treatment review/reassessment
  -> optional new TreatmentRevision
```

The accepted Treatment revision remains the source of truth for what has been authorized. `training_plans` and `training_logs` exist to make that accepted plan executable and observable.

## 2. Activation boundary

A TrainingPlan is created only from an accepted Treatment revision.

```text
Accept TreatmentRevision
  + EnsurePlanForTreatment
  -> one database transaction
```

If creation of the execution projection fails, acceptance and plan creation roll back together. Repeating the operation is idempotent by `treatment_revision_id`.

An older active TrainingPlan is superseded when a newly accepted revision becomes the current executable plan.

## 3. Current plan model

`TrainingPlan` contains:

```text
id
user_id
consultation_id?
treatment_id?
treatment_revision_id?
status
goal
duration_weeks
current_week
phases
created_at
```

The current projection builds its exercise list from accepted Treatment interventions whose kinds are executable (`exercise`, `mobility`, `self_test`).

The current feature does **not** implement a user-level `schedule_mode` switch. Historical `fixed_calendar` / `auto_shift` Profile settings were removed from the stable Profile contract and must not be treated as a current requirement.

## 4. Daily execution

Current routes:

```text
GET  /api/v1/training
GET  /api/v1/training/:id
GET  /api/v1/training/:id/today
POST /api/v1/training/:id/checkin
PUT  /api/v1/training/:id/log
GET  /api/v1/training/:id/progress
POST /api/v1/training/:id/reassess
```

The Web provides:

- current plan/focus display;
- today-task checklist;
- immersive exercise/set/rest-timer interaction;
- check-in;
- notes and structured reassessment feedback;
- progress summary.

A plan must be `active` before today-task/check-in operations are accepted.

## 5. TrainingLog and Outcome

A daily `TrainingLog` records:

```text
plan_id
user_id
treatment_revision_id?
intervention_id?
date
exercises
notes?
is_checked_in
outcome_recorded_at?
```

Check-in is not just UI progress. When the plan is linked to a TreatmentRevision, the service records a durable Outcome:

```text
kind = training_adherence
causality_level = association_only
```

Free-text/structured feedback is also converted into a durable Outcome such as:

```text
training_feedback
symptom_change
```

The system explicitly records temporal association without claiming that the intervention caused the observed change.

## 6. Reassessment and plan changes

Training feedback does **not** allow an LLM to silently replace exercises in the active plan.

```text
feedback
  -> durable Outcome
  -> current Treatment state/review policy
  -> review_recommended / paused / requires_new_diagnosis
  -> when allowed, GenerateProposal creates a new TreatmentRevision proposal
  -> user/domain acceptance required
  -> new TrainingPlan projection
```

This preserves the Treatment authority boundary:

```text
AI may propose
!= proposal accepted
!= active plan mutated
```

If the source Diagnosis is stale, the next action is a new Diagnosis rather than an automatic training mutation. If safety policy pauses Treatment, training feedback remains stored but no automatic replacement is activated.

## 7. Recovery/idempotency

The execution projection is recoverable:

- `EnsurePlanForTreatment` is idempotent for one accepted TreatmentRevision;
- `EnsureCurrentPlan` can rebuild a missing projection after a response/partial-path failure;
- feedback uses content-derived source identity to avoid duplicate Outcome creation for the same submitted payload;
- check-in has a stable per-log Outcome source key.

## 8. Progress semantics

Current progress exposes:

```text
consecutive_days
total_checkins
current_week
total_weeks
treatment_revision_id
plan_status
```

These are execution/adherence signals, not proof of clinical efficacy.

## 9. Explicitly retired assumptions

The archived feature design contained ideas that are no longer part of the current product contract:

- Profile-level `schedule_mode` with `fixed_calendar` / `auto_shift`;
- an asynchronous “log AI” that directly writes exercise substitutions into the active plan;
- arbitrary replacement-count heuristics outside Treatment review/acceptance;
- treating TrainingPlan as a second source of truth independent of TreatmentRevision.

If one of these capabilities is desired again, it requires a new product/domain decision and should be expressed through the current Treatment/Outcome authority model rather than revived from the archived document.

## 10. Acceptance criteria

- accepting a TreatmentRevision atomically creates/reuses the corresponding TrainingPlan;
- only accepted/current Treatment content becomes executable;
- a second request does not create a duplicate plan for the same revision;
- check-in records execution progress and one durable association-only Outcome;
- structured feedback records an Outcome before any reassessment proposal;
- review/safety/staleness can block automatic continuation;
- a generated replacement remains a proposal until accepted;
- current UI/API can reload the plan/log/progress after process or page restart;
- longitudinal E2E proves Treatment acceptance -> TrainingPlan -> feedback/check-in -> Outcome -> review loop.
