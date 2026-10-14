# BodySense V2 · TP-D1 multi-active TrainingPlan physical compatibility

**Owner boundary:** Cross-track issue [#268](https://github.com/BakerSean168/BodySense/issues/268) is a Track A Domain/Schema and designated Track C Backend migration dependency, verified by Track D QA. It is **not** the TP-001 Safety restriction-specific ticket and grants no new Writer.

**Reproduced against current main:** `375e5a8215100b50f45fa78f761e4a92e6154c67`, GCP Agent Fabric PG18 job `job_req_6af160737b590da17746a7507c8cc15d` exited 0. All 12 canonical migrations were applied in an isolated Docker PG18 instance. In a rollback-only synthetic transaction the legacy partial unique index `idx_training_plans_user_active` correctly rejected a second owner-identical `status=active` plan. In another rollback-only transaction, temporarily dropping that index allowed two different active plans. No shared database was changed.

**Software conflict:** `training_service.go` invokes `SupersedePlansExcept` in both an idempotent existing-projection retry and a new accepted-Treatment projection. `training_repository.go` then updates other plans to `superseded`. Dropping only the index still changes another plan without user authority. Removing only the supersede call still fails at the old index. The old single-active selector cannot silently pick an arbitrary plan once multi-active is legal.

**Next bounded implementation:** freeze source-and-target legacy Treatment versus V2 TrainingPlan semantics; serialize the physical index change with Go service and read-model changes under a single Backend/Schema Writer; retain exact TreatmentRevision idempotency. Test native PG18 multi-active, independent per-plan status, retry idempotency, transaction rollback, cross-owner access and reverse-migration conflicts. Re-creating a one-active unique index after admitting two actives is _not_ a safe automatic rollback: require an explicit operator-approved reconciliation plan.

**Release status:** all Training/Schema/Engineering/Integration/Staging/Production gates remain unapproved. Do not infer any clinical SafetyState threshold or implicitly change accepted prescriptions.

## Repeatable isolated PostgreSQL 18 acceptance probe

Track D supplies `scripts/plan/test-tp-d1-multi-active-compat.sh` plus
`scripts/plan/v2-tp-d1-multi-active-compat.test.mjs`. The native script
pins an offline PG18 image by digest, disables Docker networking,
imports **all canonical upstream migrations**, creates synthetic owners
and TrainingPlans only inside transactions, and removes its own container
on exit. It retains the independent
`idx_training_plans_treatment_revision` index for idempotency.

- `bash scripts/plan/test-tp-d1-multi-active-compat.sh` (default
  `TP_D1_EXPECTATION=legacy`) must **PASS** while reproducing the legacy
  unique-index blocker, confirming unrelated owners are isolated and
  proving two active rows can exist **only after** an index drop in a
  strictly rolled-back synthetic transaction. **This PASS is a known
  incompatibility receipt, not successful V2 admission.**
- `TP_D1_EXPECTATION=v2 bash scripts/plan/test-tp-d1-multi-active-compat.sh`
  is the **future DB acceptance mode**. It must fail on the old schema,
  and may pass only when two active plans can be inserted without the
  old unique index while preserving the TreatmentRevision index.
- Even a future V2 database-mode PASS is insufficient on its own:
  service-level implicit supersession, explicit user plan selection,
  revision idempotency, write transactions, owner authorization,
  forward/rollback migration and safety cases still require Track C/D
  acceptance. No clinical thresholds are assumed.

**Synthetic GCP replay receipt:** Agent Fabric
`job_req_10e0e37db3b0861b16ac30ba635decb9`, source
`13771d90d4903c57f4fbc4d4eefc81dbe84fc0e2` (before the test files),
exit 0. All 12 migrations applied, legacy mode passed with named
constraint evidence and isolated-drop count 2; V2 mode failed with
`TP-D1 V2 BLOCKED: legacy single-active index still exists` (expected
negative control). Every synthetic fixture container was removed.

## 2026-10-14 · Stage 1 independent-plan compatibility implementation

This stage is **shipping code with a dormant V2 compatibility option**,
not activation of the TrainingPlan V2 product or a database migration.

- `apps/api/internal/service/training_service.go` accepts the explicit
  `WithIndependentTrainingPlans()` constructor option. **No current
  application constructor enables it**, so the legacy
  `SupersedePlansExcept` behavior remains unchanged. When explicitly
  selected by a future approved caller, creating or retrying a
  projection from a pre-accepted TreatmentRevision does not supersede
  other active plans. That existing TreatmentRevision does not itself
  implement the full V2 PlanProposal/user-authorization protocol.
- `apps/api/internal/repository/training_repository.go` changes the
  legacy single-active read to a bounded owner-scoped `LIMIT 2`.
  On multiple active records it returns the identity-free
  `ErrMultipleActiveTrainingPlans` sentinel, **not** an arbitrary plan.
  The old single-plan HealthWorkspace cannot represent multiple plans;
  its consumer and the V2 UI must be upgraded to explicit Plan IDs and
  list semantics **before any V2 mode is enabled**. Until then an
  ambiguous legacy Workspace read intentionally fails closed.
- `apps/api/internal/service/training_service_test.go` tests the
  default legacy supersede path, independent projection creation,
  response-loss idempotent retry, a pre-existing revision, and
  unaccepted-revision rejection. The repository SQLMock tests check
  missing user identity, zero/one/two active rows, tenant scoping,
  identity-free errors and query-failure propagation.
- `scripts/plan/sql/tp-d1-multi-active-forward-preview.sql` and
  `tp-d1-multi-active-reverse-preview.sql` are **not located under**
  `apps/api/migrations/` and never run on application startup.
  They are future migration proposals with prerequisite checks.
  The reverse script rejects rollback if legitimate multiple-active
  plans still exist, and never selects or deactivates a user's plan.
  The independent TreatmentRevision idempotency index is preserved.
- `scripts/plan/test-tp-d1-migration-preview.sh` runs both proposals
  in a synthetic, network-isolated, pinned PG18 Docker container. It
  loads every canonical migration, proves two active plans coexist
  after the forward preview, then proves the reverse fails closed
  without changing three active plans. A separate, synthetic-only
  operator reconciliation of one sample row permits the reverse to
  complete and restores the old index. This is **not permission** to
  reconcile actual users' plans.
- `scripts/plan/v2-tp-d1-compatibility-code.test.mjs` asserts the
  explicit opt-in, fail-closed single-plan selector and offline
  preview/test harness boundaries.

**Still required for acceptance:** The dedicated Backend/Schema
Writer must coordinate the real migration with Treatment revision
idempotency, atomic creation under concurrent devices, the new
TrainingPlanRevision/Proposal runtime, owner-scoped Plan selection and
the rollback admission plan. Track D then verifies true multi-active
execution and no implicit status mutation with Go Race and PG18
up/down/recovery testing. No old `superseded` plan is silently
reactivated, and no clinical thresholds are invented.

**Release:** untouched. Do not add these previews to the live
migration chain or enable the constructor option from production
routes without a separate reviewed change and formal gates.
