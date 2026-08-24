# Off-host PostgreSQL backup & restore runbook (BS-PROD-012)

Operator-owned **off-host** disaster-recovery backups: every day a custom-format
`pg_dump` is uploaded to a private OSS/S3-compatible destination together with
its SHA-256 and a metadata object, and a freshness check alerts if backups are
missing or stale. Restore is interactive and operator-only, always into an
explicitly disposable database that is validated by the API's own validators.

## 1. Object layout

```
<OFFHOST_BACKUP_PREFIX>/<yyyyMMdd>/bodysense-postgres-<yyyyMMdd-HHmmssZ>.dump
                                   bodysense-postgres-<yyyyMMdd-HHmmssZ>.dump.sha256
                                   bodysense-postgres-<yyyyMMdd-HHmmssZ>.dump.meta.json
```

Default prefix: `bodysense/postgres`. Metadata contains `object_key`,
`checksum_sha256`, `schema_revision` (`version:dirty` from `schema_migrations`),
`created_at_utc`, `archive_bytes`, `archive_format=custom`, `pg_dump_version`,
`source {project, db_name, db_user, host}` and `retention_days`.

## 2. Configuration

Non-secret settings in `.env.production` (tracked):

| Variable | Default | Purpose |
|---|---|---|
| `OFFHOST_BACKUP_ENABLED` | `true` | master switch for the backup timer |
| `OFFHOST_BACKUP_BUCKET` | `bodysense-db-backup` | destination bucket |
| `OFFHOST_BACKUP_ENDPOINT` | `https://oss-cn-hangzhou.aliyuncs.com` | S3-compatible endpoint |
| `OFFHOST_BACKUP_REGION` | `cn-hangzhou` | signing region |
| `OFFHOST_BACKUP_PREFIX` | `bodysense/postgres` | object key prefix |
| `OFFHOST_BACKUP_URL_STYLE` | `path` | `path` or `virtual` |
| `OFFHOST_BACKUP_RETENTION_DAYS` | `30` | prune objects older than this |
| `OFFHOST_BACKUP_FRESHNESS_HOURS` | `30` | alert when the last backup is older |
| `OFFHOST_BACKUP_FRESHNESS_PROBE` | `object` | `object` (remote HEAD) or `state` (local only) |
| `OFFHOST_BACKUP_ALERT_CMD` | (empty) | command run on freshness failure |

Secrets in `.env.production.local` (untracked, host-only):

```
OFFHOST_BACKUP_ACCESS_KEY=...
OFFHOST_BACKUP_SECRET_KEY=...
```

The keys must be least-privilege (only GetObject / PutObject / DeleteObject /
ListBucket on the backup bucket). They are passed to `scripts/offhost-s3.py`
only through the process environment (`OFFHOST_BACKUP_ACCESS_KEY` /
`OFFHOST_BACKUP_SECRET_KEY`), never on the command line and never written into
artifacts. The client **refuses** `--access-key` / `--secret-key` arguments as
a security guard, so the keys can never appear in `/proc/*/cmdline` or process
listings. Process env overrides the env files so an operator can authenticate a
one-off check without editing files.

## 3. Scheduling

The deploy watcher installs and enables these units from the runtime bundle:

- `bodysense-offhost-backup.timer` — daily 02:10 (`OnCalendar=*-*-* 02:10:00`),
  runs `production-offhost-backup.sh --backup`.
- `bodysense-offhost-freshness.timer` — hourly
  (`OnCalendar=*:00:00`), runs `--check-freshness`.

Inspection:

```bash
systemctl status bodysense-offhost-backup.timer
systemctl status bodysense-offhost-freshness.timer
journalctl -u bodysense-offhost-backup.service -n 100
journalctl -u bodysense-offhost-freshness.service -n 100
```

Until `OFFHOST_BACKUP_ACCESS_KEY` / `OFFHOST_BACKUP_SECRET_KEY` are configured,
the freshness check fails hourly (there is no `last-success.json`), so an
unconfigured host alarms instead of masquerading as protected.

## 4. Manual operations

### 4.1 Run a backup now

```bash
/opt/bodysense/scripts/production-offhost-backup.sh --backup
# success prints:  OFFHOST_BACKUP_OBJECT=bodysense/postgres/<date>/bodysense-postgres-<ts>.dump
```

### 4.2 Check freshness by hand

```bash
/opt/bodysense/scripts/production-offhost-backup.sh --check-freshness
# OK:    OFFHOST_BACKUP_FRESH=OK ...
# fail:  OFFHOST_BACKUP_FRESH=FAIL reason=stale|no-last-success-state|object-probe-unreachable|...
```

Freshness failure exits non-zero and, when `OFFHOST_BACKUP_ALERT_CMD` is set,
runs `bash -c "$OFFHOST_BACKUP_ALERT_CMD"` with
`OFFHOST_BACKUP_FRESH=FAIL reason=... age_hours=... threshold_hours=...`
exported.

### 4.3 List off-host backups

```bash
# set the credential env vars from .env.production.local first
set -a; . /opt/bodysense/.env.production.local; set +a
/opt/bodysense/scripts/offhost-s3.py \
  --endpoint "$(grep OFFHOST_BACKUP_ENDPOINT /opt/bodysense/.env.production | cut -d= -f2)" \
  --bucket bodysense-db-backup --region cn-hangzhou --url-style path \
  list --prefix bodysense/postgres/
```

`offhost-s3.py` reads `OFFHOST_BACKUP_ACCESS_KEY` /
`OFFHOST_BACKUP_SECRET_KEY` from the environment only; passing
`--access-key` / `--secret-key` on the command line is refused (exit `1`,
`CliCredentialsRefused`) so secrets stay out of the process list.

## 5. Restore drill (operator-only)

The restore script is deliberately strict and interactive-gated:

```bash
# run a disposable PostgreSQL container dedicated to the drill (e.g.)
docker run -d --name restore-pg --network <bodysense_net> \
  -e POSTGRES_USER=bodysense -e POSTGRES_PASSWORD=<...> -e POSTGRES_DB=postgres \
  pgvector/pgvector:pg18
/opt/bodysense/scripts/restore-production-backup.sh \
  --object-key bodysense/postgres/20260824/bodysense-postgres-20260824-021000Z.dump \
  --target-db drill_restore_20260824 \
  --target-project drill \
  --restore-pg container:restore-pg \
  --confirm-target-isolated=yes
```

Requirements enforced by the script:

1. `--confirm-target-isolated=yes` must be supplied.
2. `--restore-pg container:<id|name>` is required and must identify a
   **disposable** PostgreSQL server/container that is provably not the live
   production postgres container/endpoint; the script resolves both to Docker
   IDs and refuses equality. All `psql`/`pg_restore` calls and `docker cp`
   operations then target that container exclusively.
3. `--target-db` must differ from production `DB_NAME` (`bodysense`).
4. `--target-project` must differ from `bodysense`.
5. `--target-db` must not already exist; it is created fresh on the disposable
   restore server and is never dropped or reused by the script.
6. The object key must live under `OFFHOST_BACKUP_PREFIX`.

What the drill does:

1. downloads `meta.json`, `*.sha256` and the dump;
2. verifies JSON validity and that `object_key`/`backup_kind` match;
3. verifies the SHA-256 sidecar strictly: it must be in `<64-hex>  <filename>`
   format, the attested filename must equal the object basename, its digest
   must equal the metadata `checksum_sha256`, and the downloaded archive's
   `sha256sum` must equal both the sidecar and the metadata (a three-way
   linked integrity check);
4. copies the archive into the disposable restore container and validates it
   with `pg_restore --list`;
5. creates the disposable target database on that container;
6. restores with `pg_restore --no-owner --no-privileges -j 2`;
7. verifies `schema_revision` in the restored database equals the backup
   metadata (`unknown`/`uninitialized` metadata is logged but not enforced);
8. runs the API's own `migration-validator` and `domain-validator` binaries
   against the restored database (default `--validator-runner docker`, or
   `--validator-runner golang` from a source checkout) by execing into the **api
   service container** that hosts `/app/validators`. The container name is
   resolved from the running Compose project — production does not set
   `container_name`, so Compose's default naming is used (`<project>-api-1`,
   e.g. `docker-api-1`); operators can pin it explicitly with
   `OFFHOST_API_CONTAINER`. The api container resolves the disposable restore
   container by its Docker network name;
9. prints `RESTORE_RESULT=PASS database=... project=... restore_pg=... object_key=...` on success.

Optional: `--baseline-version N` migrates through the published production
baseline before validation. Use `--workdir /path` to keep all artifacts
(meta/sha/archive) in an operator-specified directory for troubleshooting.

### 5.1 Safety of the drill target

The restored database is disposable and is never connected to traffic. It is
created on the explicitly supplied disposable restore container/server
(`--restore-pg container:<id|name>`), never on the production postgres
container, and the script refuses to run when the two resolve to the same
endpoint. Any restored environment that later serves traffic must run the
erasure recovery/tombstone reconciliation first (see
`docs/security/privacy-erasure-retention.md`). Production drills restore the
most recent backup and run the full validation; escalating the restored database
into service is a separate, documented operator step outside the scope of this
script.

## 6. Verification

- **Hermetic (no docker/PostgreSQL needed):** `scripts/test_offhost_s3.py`
  (specific SigV4 vectors plus a signature-verified fake S3 server, including
  refusal of command-line credentials) and `scripts/validate-offhost-dr-unit.sh`
  (23 checks: backup/retention/freshness, the restore isolation and
  `--restore-pg` guards, the SHA-256 sidecar syntax/name/digest verification,
  the env-only credential argv-leak guard, and the resolved api-container
  validation path) against stubbed PostgreSQL and the fake S3 server.
- **Docker integration:** `scripts/validate-offhost-dr.sh` runs real PostgreSQL
  18 + real `pg_dump`/`pg_restore` + the real validator binaries end to end,
  restoring into a second, disposable `restore-pg` container, including a data
  round-trip probe (`dr-probe@example.com`).
- Both are invoked from `scripts/validate-repo.sh`.

## 7. Failure playbook

| Symptom | Likely cause | Action |
|---|---|---|
| `OFFHOST_BACKUP_FRESH=FAIL reason=no-last-success-state` | no successful backup ever recorded | run `--backup`; check credentials and bucket access |
| `reason=stale` | last backup older than `OFFHOST_BACKUP_FRESHNESS_HOURS` | inspect `journalctl -u bodysense-offhost-backup.service -n 100`; run `--backup` |
| `reason=object-probe-unreachable` / `object-probe-head-failed` | OSS key rotation or bucket/permission change | verify credentials against the bucket; check endpoint/bucket in `.env.production` |
| `reason=credentials-missing-for-object-probe` | object probing without keys | add keys to `.env.production.local` |
| backup fails with `remote checksum round-trip mismatch` | upload/re-download integrity issue | rerun `--backup`; treat failure as real: do not rely on the archive |
| backup fails with `re-downloaded archive checksum does not match` | corrupted remote copy | rerun; if persistent, investigate the object store (see §7.1) |
| restore fails with `does not match the checksum sidecar` / `does not match metadata checksum_sha256` | archive or sidecar corrupted in transit or by retention | fetch the object, sidecar and metadata manually and verify (§7.1); pick a different datedir |
| restore fails with `checksum sidecar is not in '<sha256>  <filename>' format` or `does not match object key basename` | tampered or foreign sidecar paired with the archive | investigate the object store; the archive is not trusted without a valid, matching sidecar |
| restore fails `refusing to restore into the live production postgres container/endpoint` | `--restore-pg` resolved to the production postgres | supply a distinct disposable restore container/server (see §5) |
| restore fails at validators | restored schema/domain inconsistent | compare metadata `schema_revision`; check migrations manifest; escalate |

### 7.1 Verifying a specific object independently

```bash
obj=bodysense/postgres/20260824/bodysense-postgres-20260824-021000Z.dump
# fetch meta + checksum + dump manually into an operator workdir
/opt/bodysense/scripts/offhost-s3.py --endpoint ... --bucket bodysense-db-backup \
  --region cn-hangzhou --url-style path \
  get --key "$obj.meta.json" --file "/tmp/offhost-work/$(basename "$obj.meta.json")"
# ... and compare:
sha256sum "/tmp/offhost-work/$(basename "$obj")"
```

## 8. Scope boundary

Note: this runbook covers backup, retention, freshness alerting, and the
implemented, tested restore drill into an isolated disposable database.
Turning a validated restored database into a live production environment (mount
it to traffic) is an operator decision that requires the erasure-recovery
step and is intentionally left out of the automated scripts.