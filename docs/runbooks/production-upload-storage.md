# Production user upload storage cutover

Status: implementation runbook. The application defaults to local storage until an operator explicitly provisions and validates Alibaba OSS.

## Purpose and protected boundaries

`user_uploads.storage_backend + storage_key` is the durable object identity. A host filesystem path is not persisted. Go is the storage authority: OCR, Posture analysis, consultation images, deletion and privacy erasure all read/delete through `UploadStorage`. Assessment no longer reads raw image objects; it consumes completed governed Posture analysis metadata. Python receives only Go-forwarded bytes/data URLs and never receives OSS credentials or object coordinates.

The v1 upload HTTP response still exposes `file_path` for compatibility, but its value is the opaque storage key. `storage_backend` and `storage_key` remain server-private.

Do not delete the existing `api-uploads` volume during initial cutover. The local copy is intentionally retained after migration until the OSS-backed path and privacy deletion have been verified.

## Current production prerequisite

The 2026-08-24 production audit found zero `user_uploads` rows and zero upload files, so the first OSS cutover does not require moving existing health blobs. The migrator still exists for legacy/dev data and future rollback-controlled migrations. Re-check counts immediately before cutover; never assume they remain zero.

## OSS bucket requirements

Create a dedicated private bucket or a deliberately isolated private bucket prefix. For this adapter, the bucket must satisfy all of these before the API will serve with OSS configured:

- bucket ACL is `private`;
- bucket policy does not allow public access;
- bucket versioning is **disabled/unversioned**, not `Enabled` or `Suspended`;
- server-side encryption is enabled per object (`AES256` by default; KMS can be introduced later with an explicit key policy);
- the ECS role is scope-limited to `bodysense/production/uploads/`;
- the upload role must not gain write/delete authority over the PostgreSQL DR backup prefix.

Versioning is deliberately disallowed in this first adapter. With OSS versioning enabled or suspended, delete semantics can preserve previous object versions, and overwrite-prevention semantics differ. Privacy erasure must mean physical object removal under the currently implemented contract, so the API fails closed when versioning is not disabled.

## Minimum-scope ECS RAM role

Use an ECS RAM role with STS credentials. Do not place a long-lived AccessKey on the host. Replace `<bucket>` below with the real private bucket name.

The application needs bucket-level metadata reads for its startup safety check, prefix-bounded listing for privacy erasure, and object operations only within the upload prefix:

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "oss:GetBucketAcl",
        "oss:GetBucketVersioning",
        "oss:GetBucketPolicyStatus"
      ],
      "Resource": "acs:oss:*:*:<bucket>"
    },
    {
      "Effect": "Allow",
      "Action": "oss:ListObjects",
      "Resource": "acs:oss:*:*:<bucket>",
      "Condition": {
        "StringLike": {
          "oss:Prefix": [
            "bodysense/production/uploads",
            "bodysense/production/uploads/*"
          ]
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "oss:GetObject",
        "oss:PutObject",
        "oss:DeleteObject"
      ],
      "Resource": "acs:oss:*:*:<bucket>/bodysense/production/uploads/*"
    }
  ]
}
```

Do not add `oss:*`, `oss:DeleteBucket`, or permissions on the DR backup prefix. If the same ECS role carries the separate PostgreSQL DR policy, keep the two object-resource statements prefix-scoped; the DR statement intentionally has no `DeleteObject` authority.

## Host configuration before migration

Keep the default backend local while provisioning target OSS:

```dotenv
UPLOAD_STORAGE_BACKEND=local
UPLOAD_OSS_BUCKET=<private-bucket>
UPLOAD_OSS_ECS_RAM_ROLE=<ecs-ram-role-name>
# Optional only for a custom endpoint. Normally leave blank and use the
# internal endpoint selected by UPLOAD_OSS_USE_INTERNAL_ENDPOINT=true.
UPLOAD_OSS_ENDPOINT=
```

Put host-specific values in `/opt/bodysense/.env.production.local`. Do not edit tracked `.env.production` on the host. The secret/local env file is loaded after the tracked public env, so it is the production override authority.

Restart/deploy the API only after the role is attached. Startup validates bucket versioning, ACL and policy-public status and fails closed if the bucket is unsafe or the role lacks the required read actions.

## Pre-cutover checks

Use the existing production Compose identity:

```bash
cd /opt/bodysense
compose=(
  docker compose -p docker
  -f docker/docker-compose.prod.yml
  --env-file .env.production
  --env-file .env.production.local
)
```

Confirm application/schema health and current storage distribution:

```bash
"${compose[@]}" exec -T postgres \
  psql -U bodysense -d bodysense -Atc \
  "select storage_backend, count(*) from user_assets group by storage_backend order by storage_backend;"

"${compose[@]}" exec -T api /app/upload-storage-migrator \
  --from local --to oss --dry-run
```

`--dry-run` only scans the source backend and intentionally does not require the OSS target to exist. This allows scope discovery before cloud provisioning. A non-dry-run migration requires both adapters and a successful API/storage preflight.

## Copy and verify local objects

Run the migrator with the API still defaulting new writes to local:

```bash
"${compose[@]}" exec -T api /app/upload-storage-migrator \
  --from local --to oss
```

For each available asset, the migrator acquires the same asset-scoped database lock used by source removal, re-reads owner-scoped state, and commits both source and target backend/key identities to `user_asset_storage_cleanups` before copying. It then verifies source size, copies or verifies the existing target by SHA-256, verifies target size/hash, and advances the manifest with an `availability=available` compare-and-swap. After cutover it deletes the old source and confirms absence before clearing its cleanup record.

Source-removed and erased assets are never copied. A definitive CAS failure cleans and verifies the orphan target. Copy/cleanup errors retain the write-ahead inventory for safe retry; an ambiguous database outcome retains both identities. Re-running the same migration also resumes old-backend cleanup after a committed cutover. Do not remove a configured backend while its cleanup inventory remains.

Source removal makes the original unavailable first, then deletes and verifies every known source replica under the same lock. A failed Delete or failed absence check returns an error and preserves retry state; retry the source-removal command. This inventories source objects only, retains the asset and derived evidence, and is not the AS-002 all-controlled-copy verified-erasure lifecycle or receipt.

After migration:

```bash
"${compose[@]}" exec -T postgres \
  psql -U bodysense -d bodysense -Atc \
  "select storage_backend, count(*) from user_assets group by storage_backend order by storage_backend;"
```

Expected before the default-backend switch: no row that was selected for migration remains `local`.

## Switch new writes to OSS

Only after copy/verification succeeds, add the host override:

```dotenv
UPLOAD_STORAGE_BACKEND=oss
```

Then deploy/recreate the API through the normal coherent release path. Do not manually replace only the API image.

Smoke-test with a dedicated removable account:

1. upload one consultation image and one report;
2. verify the DB rows have `storage_backend='oss'`;
3. verify consultation image loading works;
4. verify OCR/report processing works;
5. verify Posture image consumption works and Assessment can reuse the resulting governed posture analysis without rereading the raw image;
6. delete one upload and prove the OSS object is gone;
7. exercise the privacy-erasure synthetic path and prove the user prefix is empty after the post-delete sweep.

Do not print object contents, STS credentials or health data into operator logs.

## Rollback

Changing `UPLOAD_STORAGE_BACKEND=local` affects **new writes only**. Existing rows continue reading from their persisted `storage_backend`, so a blind environment flip is not a data migration.

Rollback must use a compatible schema and binary that honors persisted backend identities and the cleanup inventory. Do not manually update `storage_backend` or downgrade a data-bearing schema without a separately approved, verified recovery procedure.

To return available manifests to local after OSS cutover, use an explicit reverse migration (`--from oss --to local`) after operator review. The old local copies are deleted after verified cutover, so rollback must read the current authoritative OSS objects. Source-removed assets remain unavailable and must not be restored by rollback. Keep backend adapters and their configuration until all pending source cleanup is verified; AS-002 erasure acceptance remains separate.

## Evidence required to close BS-PROD-013

The ticket remains open until real Alibaba production evidence shows:

```text
private OSS bucket safety preflight PASS
ECS RAM role / STS access PASS
local -> OSS dry-run PASS
local -> OSS copy/verify/CAS PASS (or verified zero-row no-op)
new upload persists storage_backend=oss
Consultation read PASS
OCR/Posture/Assessment read PASS
single-upload delete removes object PASS
privacy erasure removes user prefix PASS
API can be rebuilt with an empty local uploads filesystem and still read OSS-backed objects PASS
```

Local adapter/unit/integration tests are necessary but do not substitute for this production evidence.
