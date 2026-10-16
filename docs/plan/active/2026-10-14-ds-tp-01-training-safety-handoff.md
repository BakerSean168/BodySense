# DS-TP-01 · TrainingPlan / Trainer / SafetyState contract handoff

Status: **DRAFT / Track A only / NOT a deployed API, database gate, or medical rule.**

Owner: Domain & Schema (isolated Track A checkout). Engineering ownership: Track C. Acceptance owner: Track D.

Baseline: approved `docs/product/domain/training.md`, `docs/product/domain/safety.md`, `docs/product/acceptance.md`; master `2026-10-13-v2-parallel-master.md`. This document MUST NOT override these sources.

## 1. Persistence ownership and invariants

- `TrainingPlan(planId,userId)` has multiple independently active instances. `TrainingPlanRevision(revisionId,planId,parentRevisionId)` is an immutable accepted content snapshot. `PlanProposal(proposalId,contentFingerprint,basedOnRevisionId)` remains pending until exact user authorization. Proposal preview is never a persisted accepted plan or a training completion.
- `PlanSessionTemplate(templateId)` belongs to a plan revision. `PlanExercise(planExerciseId,exerciseId)` distinguishes the prescription line from its standard exercise concept. A name change does not change the safety matching concept.
- `TrainingSessionLog(sessionLogId,userId,planId,planRevisionId,templateId,startedAt,trainingDate,timeZoneSnapshot)` is a distinct execution instance. `SessionAction(actionId,sessionLogId,planExerciseId,setNumber?,scope,kind,correctsActionId?,sourceId)` is append-only. Same-day repeated execution creates separate `sessionLogId`; no `(planId,date)` uniqueness.
- `SafetyState` is owned ONLY by BodyState; `SafetyRestriction(restrictionId,userId,scope,riskLevel,liftPolicy,sourceIds)` requires durable restriction-specific changes and audit. `TrainerGate` is a computed read projection, not a second persistent authority. Safety changes cannot silently mutate plan state, plan revision, or past SessionAction facts.
- `BodyFact`, `SymptomEpisode`, `BodyObservation`, and `ConditionEntry` remain separately owned; read links use owner-bound IDs and evidence versions. Never introduce `BodyConcern`.

## 2. Named ViewModels / queries — _target only_

| Query / shape                               | Stable fields                                                                                                                                                                      | Deliberate negative or freshness states                                                                                        |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `ListAcceptedTrainingPlans`                 | `planId,title,goal,status,currentAcceptedRevisionId,sessionTemplateSummaries[],relatedConditionIds[],relatedBodyRefs[]`                                                            | loading, empty, denied, stale; no proposal masquerading as accepted                                                            |
| `GetTrainingPlanDetail(planId)`             | plan revision, schedule mode (`frequency_only` or user-authorized `calendar`), templates/stages/exercises with `planExerciseId` and `exerciseId`, proposal/impact hints separately | revision conflict, source unavailable, needs_review, no assumed calendar or diagnosis                                          |
| `GetPlanProposal(proposalId)`               | kind, target plan/revision, exact diff/content fingerprint, evidence refs, deadline, state                                                                                         | expired, rejected, conflicting evidence/revision, pending after dismiss; cannot indicate accepted without receipt              |
| `GetTrainerSession(sessionLogId)`           | fixed planRevisionId/templateId, timeZoneSnapshot, trainingDate, execution state, effective per-set actions, correction history, pause/ended reason                                | recoverable pending/paused, permission denied, stale, failed/unavailable safety gate; no inferred completion                   |
| `GetCurrentTrainerGate(planId,exerciseId?)` | result, matchedRestrictionIds, reasons, requiresReview, exact evidence/revision checked                                                                                            | unavailable is **not allow**; missing exercise identity or source prevents claiming safety; no medical severity inferred by UI |
| `ListSafetyRestrictions`                    | restrictionId, riskLevel, scope, validFrom/Until, liftPolicy/conditions, liftBasis?, source references and availability, derived active/review/lifted display                      | source unavailable, clearance unverified, expired review-only, detached/erased source; no fabricated clinician clearance       |

Public UI does not expose raw technical revisions as clinical data. The Go command must still require a server-issued expected revision and return a durable commit receipt.

## 3. Commands, authorization and write boundary — _target shapes; field transport to be agreed with Track C_

| Command               | Preconditions (Go transactional owner)                                                                                             | Successful receipt                                                        | Must reject / must not write                                                                                                                       |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| AcceptPlanProposal    | exact user, proposalId, fingerprint, expected plan revision (for update), current evidence/Safety context, idempotency key         | planId, acceptedRevisionId, userDecisionId, persisted domain revision     | expired after **Training** 7×24 h, modified base revision, changed relevant safety evidence, unauthorized/cross-user, duplicate mismatched payload |
| ApplyUserPlanEdit     | explicit target plan, stable exercise/template IDs, authorized patch, expected plan revision                                       | new immutable revision and idempotent receipt                             | agent suggestion alone, cross-plan accidental patch, unsourced implicit edit                                                                       |
| StartTrainerSession   | accepted plan/template revision; user explicitly starts; verified current Safety gate; one in_progress per user                    | new sessionLogId, fixed revision, IANA timezone snapshot and local date   | mere preview, competing in_progress, safety unavailable/block                                                                                      |
| AppendSessionAction   | exact active session, explicit user action, planExerciseId + scope/set number, expected session revision, idempotency              | actionId and session revision                                             | navigation/video/close/disconnect creating completed or skipped; foreign session; no durable source                                                |
| PauseResumeEndSession | explicit user action or verified safety transition, exact session/revision                                                         | updated session state with pause/ended reason, unchanged existing actions | close/network loss as finished, safety unavailable treated as allow, auto-completing remaining sets                                                |
| LiftSafetyRestriction | exact user/restrictionId, expected SafetyState revision, matching liftPolicy and liftBasis with admissible provenance, idempotency | restriction-specific lift event, revision, actor/time/receipt             | expiry of non-on_expiry, unadmitted OCR, unsourced doctor claim, agent downshift, stale revision, concurrent higher-risk overwrite                 |

The following are **not** write authorization: AI draft, elapsed schedule day, exercise video playback, next/previous button, external source correction by itself, old TrainingLog `is_checked_in`, or an offline cached gate.

## 4. Read-time trainer gate decision boundary

- Evaluate **currently effective** restrictions at the Go authority on start/resume/before affected action and after SafetyState changes. Match `all_training`, plan-scoped restrictions, and standard `exerciseId` (optionally plan-scoped), not exercise display text or `planExerciseId`.
- Preserve all overlapping restriction IDs and reasons. `urgent` blocks globally; matching `restricted` blocks; matching `advisory` warns. Removing one restriction MUST NOT remove another. These product levels do not constitute validated clinical classification.
- `on_expiry` is only valid with a verified deadline and rule; expiry records an idempotent restriction-specific `liftBasis=expiry`. `user_reverification` and `professional_clearance` merely prompt review at their validUntil and remain effective until evidence-backed explicit lift.
- Gate error, ambiguous action identity, unavailable clinical provenance, or uncertain rule applicability must never be represented as `allow`. Do not silently classify a symptom as global urgent or set a medical threshold in this contract.
- Matching `restricted` mid-session yields `paused/safety_hold`; `urgent` yields `ended/safety_stopped`. Both preserve prior SessionAction facts; no automatic skipped/completed actions, plan status changes or revisions.

## 5. Legacy migration and exact negative acceptance backlog

| Case                  | Reproduction                                                                           | Required result                                                                            | Engineering owner          |
| --------------------- | -------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | -------------------------- |
| TP-17 / TP-23         | two user-accepted plans, both active                                                   | second accept must not supersede first; reopening completed retains plan identity          | Go Training + migration    |
| TP-20 / TP-13         | same plan twice per local day; session crossing midnight in user IANA zone             | separate sessionLogIds; trainingDate frozen at session start, planRevisionId fixed         | Go Trainer + PostgreSQL    |
| TP-11 / TP-12 / TP-21 | navigate to last action, show demo, disconnect then resume                             | zero completed/skipped actions absent direct user action; corrections append, not rewrite  | Go Trainer + React E2E     |
| TP-22                 | old accepted proposal fingerprint after another device modifies base / relevant safety | reject without changing any plan; replay matching valid request idempotent                 | Go PlanProposal            |
| SAFE-04 / SAFE-05     | advisory + restricted same concept, urgent globally; lift one                          | remaining stricter restriction blocks; preserve all reason/source evidence                 | Go SafetyState             |
| SAFE-02 / SAFE-09     | non-expiry restriction at review time; unsupported OCR clearance                       | no lift; requires verified source/context                                                  | Go SafetyState + UserAsset |
| SAFE-08               | two devices: old evidence correction and new urgent restriction concurrently           | restrictionId-conditional CAS; both restrictions/history preserved; no full JSON overwrite | Go SafetyState + PG18 race |
| SAFE-10               | deny/timeout SafetyState fetch just before Start/Resume/Next                           | reject affected execution without false safe status or plan mutation                       | Go gate + React            |
| SAFE-06 / SAFE-07     | new restricted then urgent arrives mid-session                                         | safety_hold or safety_stopped; preserve actual actions, no auto-skipped set                | Go Trainer + E2E           |
| SEC-01                | use another user's planId, sessionLogId, restrictionId                                 | owner-hidden unauthorized response, no mutation or health data leak                        | Go HTTP + PG18             |
| TP-16                 | show/dismiss proposed plan without acceptance                                          | no accepted plan, revision, or completed record                                            | Go + React                 |

Migration limitations: old `TrainingLog.is_checked_in` is **not** per-set proof. Old `Treatment` pause/review flags cannot map directly to user-managed TrainingPlan statuses. Never alter historical source or already executed immutable revisions during conversion. Rollback requires synthetic-data replay and audit of source and action retention before the `database` gate.

### 5.1 Trainer SessionAction / IANA execution reference (2026-10-14)

The companion `scripts/plan/v2-trainer-actions-model.test.mjs` exercises **11 synthetic tests** against an intentionally bounded append-only reference. It models a user-started session with an accepted plan/template and current Go-verified safety decision, a frozen IANA `trainingDate`, a unique per-execution `sessionLogId`, source-bound per-set or whole-exercise actions, immutable correction records, exact revision/idempotency replay, and a no-fabrication safety transition. Reject cases cover draft/foreign plan, unavailable or blocked gate, concurrent `in_progress` session, navigation/video/close/disconnect, missing or conflicting action identity, cross-user, stale revision, changed replay, double correction, and missing pre-action safety verification.

**Limits:** This is NOT a durable event store, OpenAPI DTO, Go implementation, clinical provenance verifier or transaction. In particular, synthetic `verifiedGate` and `sourceId` inputs stand for Go-admitted facts; only Track C can establish their authority. There is no inferred calendar schedule, clinical threshold, or implicit action from a UI event. Native PostgreSQL race/rollback and end-to-end actions remain required before either gate may advance.

## 6. Track C blockers / evidence required for gate progression

Current code audit points to `apps/api/internal/service/training_service.go` calling `SupersedePlansExcept`, server-local `localDate(time.Now())`, and `apps/api/internal/service/body_state_service.go` calling `SetSafetyState(ctx,userID,nil,state,"consultation")` with whole-JSON replace in `apps/api/internal/repository/body_state_repository.go`. The legacy `apps/api/internal/service/training_service_test.go::TestEnsurePlanForTreatmentIsIdempotent` specifically expects `supersedeCalled=true`; Track C must retire that incompatible expectation and test two independently active plans, including a repeated restore of one without mutating the other. These are **static migration gaps**, not a recorded concurrent PostgreSQL reproduction.

Track C must first agree typed transport / ID / expectedRevision and source provenance, implement additive SQL and authority-bound Go transactions in its owned checkout once writer is free; Track D must then run isolated PostgreSQL 18 up/down/up, concurrent revision/idempotency/tenant/security, supported negative E2E and exact-SHA CI. Do not mark `view_model`, `database`, `engineering` or `integration` ready based on this conceptual handoff.

## 7. Scope withheld and explicit outstanding decisions

No arbitrary clinical risk thresholds, contraindication model, OCR clinical admission policy, medical release assertion or Diagnosis Q-17 TTL is introduced. Training proposal's approved 7×24h expiration is **not** a Diagnosis proposal deadline. Physical SQL layout, user verified timezone fallback behavior, generated endpoint DTOs and authoritative risk-rule version registry remain implementation/design work; any unresolved medical rule remains fail-closed until qualified approval.

Evidence target: `docs/product/domain/training.md` TP-D1–D12, `docs/product/domain/safety.md` SAFE-D1–D8, `docs/product/acceptance.md` TP-11–25 / SAFE-01–10 / SEC-01. This document alone is insufficient to advance any domain gate.
