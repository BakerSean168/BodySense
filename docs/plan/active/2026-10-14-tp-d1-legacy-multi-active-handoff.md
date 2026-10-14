# BodySense V2 · TP-D1 multi-active TrainingPlan physical compatibility

**Owner boundary:** Cross-track issue [#268](https://github.com/BakerSean168/BodySense/issues/268) is a Track A Domain/Schema and designated Track C Backend migration dependency, verified by Track D QA. It is **not** the TP-001 Safety restriction-specific ticket and grants no new Writer.

**Reproduced against current main:** `375e5a8215100b50f45fa78f761e4a92e6154c67`, GCP Agent Fabric PG18 job `job_req_6af160737b590da17746a7507c8cc15d` exited 0. All 12 canonical migrations were applied in an isolated Docker PG18 instance. In a rollback-only synthetic transaction the legacy partial unique index `idx_training_plans_user_active` correctly rejected a second owner-identical `status=active` plan. In another rollback-only transaction, temporarily dropping that index allowed two different active plans. No shared database was changed.

**Software conflict:** `training_service.go` invokes `SupersedePlansExcept` in both an idempotent existing-projection retry and a new accepted-Treatment projection. `training_repository.go` then updates other plans to `superseded`. Dropping only the index still changes another plan without user authority. Removing only the supersede call still fails at the old index. The old single-active selector cannot silently pick an arbitrary plan once multi-active is legal.

**Next bounded implementation:** freeze source-and-target legacy Treatment versus V2 TrainingPlan semantics; serialize the physical index change with Go service and read-model changes under a single Backend/Schema Writer; retain exact TreatmentRevision idempotency. Test native PG18 multi-active, independent per-plan status, retry idempotency, transaction rollback, cross-owner access and reverse-migration conflicts. Re-creating a one-active unique index after admitting two actives is *not* a safe automatic rollback: require an explicit operator-approved reconciliation plan.

**Release status:** all Training/Schema/Engineering/Integration/Staging/Production gates remain unapproved. Do not infer any clinical SafetyState threshold or implicitly change accepted prescriptions.
