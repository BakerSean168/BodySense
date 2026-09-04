# BodySense Data Durability & Backup

**Status:** ACTIVE / PARKED BY OWNER  
**Created:** 2026-08-25  
**Source:** split from archived `production-security-runtime-knowledge-closeout-2026-08-23.md` tickets BS-PROD-012 and BS-PROD-013.

## 1. Outcome

Keep the remaining production data-durability work explicit without blocking ordinary BodySense development or falsely marking cloud-side backup acceptance as complete.

The plan has two independent durability outcomes:

1. PostgreSQL has an independently restorable **off-host** backup with a real restore drill.
2. User-uploaded health objects no longer depend on the Alibaba ECS local upload volume and can be erased through the same privacy boundary.

Until the owner reactivates this plan, production intentionally keeps the current PostgreSQL/local-upload arrangement. Repository-side implementations remain preserved and must not be removed as “unused”.

**2026-09-04 parked-state reconciliation:** the legacy AccessKey-based `bodysense-offhost-*` automatic timers are retired and disabled while this plan is parked. Their scripts/units remain packaged only for manual compatibility and historical rollback evidence. The supported PostgreSQL DR path is `production-postgres-dr.sh` with ECS RAM Role + private OSS. Its unit files are now bootstrap-installed idempotently, but `install-production-dr.sh` leaves all backup/status/restore timers disabled while `DR_ENABLED=false`. No real Alibaba off-host backup or restore-drill acceptance is claimed by this reconciliation.

## 2. Protected contracts

- Alibaba ECS (`body.bakersean.top`) remains the sole production runtime.
- GCP-dev remains the development and production-operations control plane.
- Production secrets stay untracked and are never committed to Git or embedded in OCI images.
- Database restore evidence must prove migration identity and domain semantics, not only that a dump file exists.
- Upload storage remains private and owner-scoped; no public bucket/CDN shortcut is allowed for health data.
- Privacy erasure must delete database references and stored objects through the same durable deletion boundary.
- Existing local adapters remain available as a rollback/development seam until real cloud cutover is accepted.

## 3. Current preserved implementation

### PostgreSQL DR

Repository support already exists for:

- `production-dr-manager`;
- logical custom-format PostgreSQL backup + SHA-256/manifest semantics;
- ECS RAM-role-only production OSS authentication;
- freshness/status checks;
- daily backup / weekly restore drill / status timers;
- pre-mutation deployment backup gate;
- isolated restore drill with migration/domain validation.

Local/production-shaped evidence is valid, but **real Alibaba off-host object evidence does not exist yet** because the required private bucket/RAM role has not been provisioned.

### User-upload durability

Repository support already exists for:

- `UploadStorage` abstraction and local + OSS adapters;
- storage identity (`storage_backend + storage_key`);
- upload/OCR/Posture/Assessment/Consultation reads through the storage port;
- privacy-erasure object deletion;
- local → OSS migrator with size/SHA-256 verification and compare-and-swap semantics;
- production startup fail-closed checks for private, unversioned OSS configuration.

The last production audit had no user-upload rows/files requiring migration, but production still uses the local backend.

## 4. Tickets

## BS-DUR-001 — Provision and prove off-host PostgreSQL disaster recovery

**Goal:** a real production PostgreSQL backup exists outside the ECS host and an isolated restore drill proves it is usable.

**Scope:** private OSS/S3-compatible bucket, ECS RAM role, real backup/status/restore drill, timer activation, restore evidence.

**Out of scope:** PITR/replication, multi-region database architecture, changing PostgreSQL major version.

**Implementation when reactivated:**

1. Provision a private backup bucket and least-privilege ECS RAM role without long-lived object-store keys on the host.
2. Configure only untracked production-local identifiers/secrets required by the existing DR manager.
3. Run a real production backup and verify remote object metadata + SHA-256 manifest.
4. Run the isolated restore drill from the remote object.
5. Require exact migration state and `DOMAIN_SEMANTICS=PASS`.
6. Enable/verify scheduled backup/status/restore-drill timers.
7. Record bucket policy/retention, object identity, restore timestamp and validator output without recording secrets.

**Acceptance:** remote backup freshness passes; restore drill completes from the off-host object; migration/domain validation passes; no drill database remains; production public health remains green.

**Rollback/containment:** do not delete existing local backups; disable the timers/remote backend if cloud policy is misconfigured and retain the last verified local recovery path.

## BS-DUR-002 — Cut production user uploads to durable private object storage

**Goal:** production upload/read/OCR/Posture/Assessment/delete/privacy-erasure behavior is independent of the ECS upload volume.

**Scope:** private upload bucket, ECS RAM role, OSS backend configuration, zero-or-real-data migration, end-to-end validation.

**Out of scope:** public asset CDN, arbitrary user file sharing, replacing the application storage abstraction.

**Implementation when reactivated:**

1. Provision a distinct private upload bucket and least-privilege ECS RAM role/policy.
2. Re-audit production rows/local objects immediately before cutover.
3. If data exists, run copy → hash/size verify → manifest compare-and-swap; retain local source copies for rollback until acceptance.
4. Switch production to `UPLOAD_STORAGE_BACKEND=oss` through the normal coherent release/runtime configuration path.
5. Exercise upload → read → OCR → Posture/Assessment fallback → delete.
6. Run privacy erasure and prove prefix pre/post sweeps remove all user objects.
7. Verify behavior after API container restart and with the local upload volume unavailable to the application path.

**Acceptance:** all protected user-upload flows pass against the private object backend; database storage identity matches remote objects; privacy erasure leaves no user prefix; production health remains green.

**Rollback/containment:** keep the verified local source copy and local adapter until cutover acceptance; revert configuration only when storage identity/data consistency is proven safe.

## BS-DUR-003 — Durability closeout review and archive

**Goal:** close this plan only after both real cloud-side durability outcomes are proven.

**Dependencies:** BS-DUR-001 and BS-DUR-002.

**Verification:**

```bash
pnpm verify:release
pnpm validate:local-deploy
git diff --check
```

Then run the real Alibaba backup/restore and upload-object-store production smoke paths described above.

**Acceptance:** both tickets have real production evidence; no P0/P1 durability/privacy finding remains; this file moves to `docs/plan/archive/`.

## 5. Reactivation triggers

Reactivate this plan when any of these occurs:

- production begins storing meaningful user upload/health-file volume;
- database/user data becomes costly or unacceptable to recreate;
- the current single-host durability risk is no longer acceptable;
- compliance/privacy requirements demand independent backup/retention controls;
- the owner decides the OSS/RAM-role operational complexity/cost is justified.

No other BodySense feature work should be blocked merely because this parked plan remains active.
