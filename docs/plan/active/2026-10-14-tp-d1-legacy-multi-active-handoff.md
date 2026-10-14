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
