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
| `OFFHOST_BACKUP_OBJECT_ACL` | `private` | object ACL on uploaded backup objects; the client refuses any value other than `private` or empty |
| `OFFHOST_BACKUP_SSE` | `AES256` | PutObject server-side encryption (`AES256` or `aws:kms`); empty sends no SSE header (only for buckets with default SSE) |
| `OFFHOST_RECOVERY_MODE` | `false` | let a restore proceed when production is down (skips only the production-side isolation proofs; see §5) |
| `OFFHOST_RECOVERY_PRODUCTION_PROJECT` | `bodysense` | production compose project name used in recovery mode |
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

### Policy invariants

- **Retention is apply-or-fail.** Before recording a success, the backup lists
  every object under `OFFHOST_BACKUP_PREFIX` and prunes objects older than
  `OFFHOST_BACKUP_RETENTION_DAYS` (the newest day directory is never pruned).
  If that object listing cannot be fetched, the backup aborts with
  `off-host retention listing failed; refusing to record last success` and
  `last-success.json` is **not** updated — a silent retention skip can never
  leave the freshness check reporting OK while sensitive backups accumulate
  without a proven retention bound.
- **The schema revision is verified fail-closed on both sides.** The backup
  reads `<version>:<dirty>` from `schema_migrations` and refuses to record a
  success if the table is missing/uninitialized, empty, or the query fails —
  a dump is never marked successful while carrying an unverified revision. The
  restore refuses archive `meta.json` objects whose `schema_revision` is
  `unknown`/`uninitialized`/empty up front, and requires the restored database's
  revision to equal the metadata revision exactly before reporting PASS.
- **Destination privacy is proven, never assumed.** Every backup first runs a
  fail-closed private-destination preflight: the bucket ACL must be readable and
  provably private (a public or unreadable ACL, or a `GetBucketPolicyStatus`
  result of `IsPublic=true`, aborts the backup before any object is uploaded and
  after a failed run the previous `last-success.json` is left untouched). Every
  uploaded object then carries `x-amz-acl=private` — the client refuses any other
  object ACL — and `x-amz-server-side-encryption` (`AES256` by default).
  A store that does not implement `GetBucketPolicyStatus` is accepted with a
  warning: the provably-private ACL remains the privacy proof.
- **Freshness is compared in whole seconds.** A backup is stale when
  `now - last_success_at_utc` exceeds `OFFHOST_BACKUP_FRESHNESS_HOURS * 3600`
  seconds (no whole-hour truncation: with a 30h policy, a 30h59m-old backup
  reports `age_hours=30.983` and is stale). A future-dated
  `last_success_at_utc` (host clock skew or tampered state) is rejected as
  `reason=future-dated-last-success` and never treated as fresh.

## 3. Scheduling

The deploy watcher installs and enables these units from the runtime bundle:

- `bodysense-offhost-backup.timer` — daily 02:10 Asia/Shanghai
  (`OnCalendar=*-*-* 02:10:00 Asia/Shanghai`), runs
  `production-offhost-backup.sh --backup`.
- `bodysense-offhost-freshness.timer` — hourly
  (`OnCalendar=*-*-* *:00:00 Asia/Shanghai`), runs `--check-freshness`.

The timezone is **embedded in the `OnCalendar=` expression** (systemd syntax
`<calendar> <timezone>`); there is no bare `Timezone=` directive, which is
non-standard in `[Timer]` and ignored/overridden by systemd. The documented
02:10 Asia/Shanghai schedule is therefore guaranteed regardless of the host's
configured timezone.

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
# run a disposable PostgreSQL container dedicated to the drill on its OWN
# dedicated non-host drill network (never attached to the production postgres
# network), and declare that it is a disposable drill target for
# --target-project drill: the drill network itself must be declared on the
# container (bodysense.restore-network) so the isolation proof is not just a
# fortuitous lack of overlap with production
# give the drill resources names unique to this run (never reuse a box name),
# and put the disposable restore server on its own dedicated non-host drill
# network; use a name like restore-pg-<suffix> to avoid colliding with any
# existing container
suffix="$(date +%s)"
drill_net="bodysense-drill-net"
restore_pg="restore-pg-$suffix"
docker network create "$drill_net"
docker run -d --name "$restore_pg" --network "$drill_net" \
  --label bodysense.restore-project=drill \
  --label bodysense.disposable-restore=yes \
  --label bodysense.restore-network="$drill_net" \
  -e POSTGRES_USER=bodysense -e POSTGRES_PASSWORD=<...> -e POSTGRES_DB=postgres \
  pgvector/pgvector:pg18

# the api container hosts the validator binaries; the script execs into it to
# run them against the disposable restore database, so it must be attached to
# the drill network as well (reachability is one-way: the restore container
# stays off the production network).
OFFHOST_API_CONTAINER=docker-api-1
docker network connect "$drill_net" "$OFFHOST_API_CONTAINER"
# Evidence is captured BEFORE teardown, then the drill is fully dismantled even
# on failure: disconnect the api container, remove the disposable restore
# container and its dedicated drill network, so nothing disposable survives the
# drill (a leftover restore-pg box or network would be a future isolation risk).
evidence_dir="$HOME/bodysense-drills/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$evidence_dir"
cleanup() {
  docker network disconnect "$drill_net" "$OFFHOST_API_CONTAINER" || true
  docker rm -f "$restore_pg" || true
  docker network rm "$drill_net" 2>/dev/null || true
}
trap cleanup EXIT

set -o pipefail
/opt/bodysense/scripts/restore-production-backup.sh \
  --object-key bodysense/postgres/20260824/bodysense-postgres-20260824-021000Z.dump \
  --target-db drill_restore_20260824 \
  --target-project drill \
  --restore-pg "container:$restore_pg" \
  --confirm-target-isolated=yes 2>&1 | tee "$evidence_dir/restore.log"
```

Requirements enforced by the script:

1. `--confirm-target-isolated=yes` must be supplied.
2. `--restore-pg container:<id|name>` is required and must identify a **running,
   disposable** PostgreSQL container that is provably isolated from the live
   production postgres container/endpoint. The proof is fail-closed via `docker
   inspect` — an inspection/parsing failure is a refusal, never an empty
   "isolated" result — and refuses when:
   - the restore container resolves to the same Docker ID as the production
     postgres container;
   - the restore container belongs to the production Compose project
     (`com.docker.compose.project` label equal);
   - the restore container uses host networking (`HostConfig.NetworkMode=host`)
     or no networking (`none`): a host-network target still reaches
     host-published production endpoints, so it is not provably isolated
     regardless of its Docker-network set;
- the restore container is attached to any Docker network the production
     postgres container is attached to (so drill servers run on their own
     network, never on the production postgres network), or is attached to
     any network beyond its declared `bodysense.restore-network` (the
     only-network rule: the declared drill network must be the container's
     sole network);
   - the restore container publishes any host port (`HostConfig.PortBindings`
     non-empty): a host-published target is reachable from the host and not
     provably isolated regardless of its Docker-network set;
   - the container does not declare `bodysense.restore-network=<network>` —
     the dedicated drill network must be declared on the container itself, never
     merely left to a fortuitous absence of overlap with production;
   - the declared `bodysense.restore-network` is `host`/`none`, or is not
     actually a network the container is attached to;
   - either side's network set cannot be inspected/parsed (refused as
     unprovable, not treated as empty);
   - the restore container is not running;
   - the restore container does not declare labels
     `bodysense.restore-project=<--target-project>` and
     `bodysense.disposable-restore=yes`.
   All `psql`/`pg_restore` calls and `docker cp`
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
   metadata exactly — the metadata gate is fail-closed: `meta.json` declaring a
   `schema_revision` of `unknown`/`uninitialized`/empty is refused up front,
   before any archive download, and the post-restore equality check is always
   enforced;
8. runs the API's own `migration-validator` and `domain-validator` binaries
   against the restored database (default `--validator-runner docker`, or
   `--validator-runner golang` from a source checkout) by execing into the **api
   service container** that hosts `/app/validators`. The container name is
   resolved from the running Compose project — production does not set
   `container_name`, so Compose's default naming is used (`<project>-api-1`,
   e.g. `docker-api-1`); operators can pin it explicitly with
   `OFFHOST_API_CONTAINER`. The api container must be attached to the drill
   network as well as the production network so it can reach the disposable
   restore container by its Docker network name (never the other way around: the
   restore container stays off the production network);
9. prints `RESTORE_RESULT=PASS database=... project=... restore_pg=... object_key=...` on success.

The database password never appears on a process command line: validators
receive it only via `PGPASSWORD` in their environment — inherited directly by
the golang runner, or injected through a mode-0600 `--env-file` on the `docker
exec` path (which also keeps it out of the `docker` CLI argv). The
`-database-url` passed to the validators contains no password at all.

Optional: `--baseline-version N` migrates through the published production
baseline before validation. Use `--workdir /path` to keep all artifacts
(meta/sha/archive) in an operator-specified directory for troubleshooting.

### 5.1 Safety of the drill target

The restored database is disposable and is never connected to traffic. It is
created on the explicitly supplied disposable restore container/server
(`--restore-pg container:<id|name>`), never on the production postgres
container, and the script refuses to run unless `docker inspect` proves that
container is running, attached **only** to its declared dedicated non-host drill
network (never to any Docker network the production postgres is on, never
beyond its declared drill network, never to the `host`/`none` drivers, and
publishing no host ports), outside the production Compose project, distinct
from the production postgres container, and labelled
`bodysense.restore-project=<target>` + `bodysense.disposable-restore=yes` +
`bodysense.restore-network=<drill network>`. Network and port inspection is
fail-closed:
an inspection/parsing error is a refusal, never an empty "shares no network"
result. Any restored environment that later serves traffic must run the
erasure recovery/tombstone reconciliation first (see
`docs/security/privacy-erasure-retention.md`). Production drills restore the
most recent backup and run the full validation; escalating the restored database
into service is a separate, documented operator step outside the scope of this
script.

### 5.2 Recovery mode (production is down)

When production is down there is no live production postgres container to
compare against, so the production-side proofs (container-ID difference,
shared-network intersection, discovered Compose project) **cannot be made**.
Recovery mode — `--recovery-mode=yes` (or `OFFHOST_RECOVERY_MODE=true`) —
skips exactly those proofs and is therefore **strictly weaker isolation
evidence** than a normal drill. It is opt-in and refused by default; the
production project name is taken from `--recovery-production-project` /
`OFFHOST_RECOVERY_PRODUCTION_PROJECT` (default `bodysense`) so the target-side
proofs still compare against the real production project. Every target-side
proof still applies unchanged: running, non-host/non-none network mode,
attached only to the declared dedicated drill network, no published host ports,
`bodysense.restore-project=<target>`/`bodysense.disposable-restore=yes`/
`bodysense.restore-network` labels, and a `com.docker.compose.project` label
that differs from the declared production project. Run it exactly like §5 (the
recovery-mode example restore container must declare
`bodysense.restore-project=<target-project>`):

```bash
/opt/bodysense/scripts/restore-production-backup.sh \
  --object-key bodysense/postgres/20260824/bodysense-postgres-20260824-021000Z.dump \
  --target-db recovery_restore_20260824 \
  --target-project drill \
  --restore-pg "container:$restore_pg" \
  --confirm-target-isolated=yes \
  --recovery-mode=yes
```

Afterwards, run a normal drill again (the production-side proofs restored) and
compare both logs before the recovered environment is put back into service.

## 6. Verification

- **Hermetic (no docker/PostgreSQL needed):** `scripts/test_offhost_s3.py`
  (specific SigV4 vectors plus a signature-verified fake S3 server, including
  refusal of command-line credentials, the fail-closed private-destination
  preflight against public/unreadable ACLs and public policy-status, and the
  `x-amz-acl=private` + SSE wire headers) — 30 checks — and
  `scripts/validate-offhost-dr-unit.sh`
  (64 checks: backup/retention/freshness, per-mode lock independence (backup and
  freshness can no longer mask each other), the fail-closed schema-revision gates
  on both the backup and restore sides, the private-destination preflight
  aborting before any upload on a public/unreadable bucket ACL or public policy
  status, the object ACL/SSE upload headers, the restore isolation guards — ID
  equality, running state, production-Compose-project membership, host/none
  networking refusal, declared-drill-network enforcement, the only-network rule
  (no networks beyond the declared drill network), the no-published-host-ports
  rule, fail-closed shared-network and network-enumeration refusal,
  disposable-label declaration — the
  `--restore-pg` guards, the SHA-256 sidecar syntax/name/digest verification,
  the DB password argv-leak guard (env-only `PGPASSWORD`, never in
  `-database-url`/argv), the resolved api-container validation path, recovery
  mode (restore with production down) with its refusals for a target equal to or
  labelled with the production project, and the
  systemd timers' timezone-in-`OnCalendar` contract) against stubbed PostgreSQL
  and the fake S3 server.
- **Docker integration:** `scripts/validate-offhost-dr.sh` runs real PostgreSQL
  18 + real `pg_dump`/`pg_restore` + the real validator binaries end to end,
  restoring into a second, disposable `restore-pg` container, including a data
  round-trip probe (`dr-probe@example.com`).
- Both are invoked from `scripts/validate-repo.sh`.

## 7. Failure playbook

| Symptom | Likely cause | Action |
|---|---|---|
| `OFFHOST_BACKUP_FRESH=FAIL reason=no-last-success-state` | no successful backup ever recorded | run `--backup`; check credentials and bucket access |
| `reason=stale` | last backup older than `OFFHOST_BACKUP_FRESHNESS_HOURS` (compared in whole seconds; no whole-hour truncation) | inspect `journalctl -u bodysense-offhost-backup.service -n 100`; run `--backup` |
| `reason=future-dated-last-success` | host clock skew or tampered state, or a broken state write | check host clock (`date -u`); fix or replace `last-success.json`; a future-dated success is never trusted as fresh |
| `reason=object-probe-unreachable` / `object-probe-head-failed` | OSS key rotation or bucket/permission change | verify credentials against the bucket; check endpoint/bucket in `.env.production` |
| `reason=credentials-missing-for-object-probe` | object probing without keys | add keys to `.env.production.local` |
| backup fails with `remote checksum round-trip mismatch` | upload/re-download integrity issue | rerun `--backup`; treat failure as real: do not rely on the archive |
| backup fails with `off-host retention listing failed; refusing to record last success` | object-store listing failed (endpoint, ListBucket permission, network) during pruning | verify ListBucket access/endpoint; rerun `--backup`; `last-success.json` was **not** advanced, so retention stayed bounded by the previous proof |
| backup fails with `no schema_migrations table` / `schema_migrations query failed` / `schema_migrations exists but has no rows` | migration state is missing/unreadable/empty | fix migrations and migration state on the source DB, then rerun `--backup`; no backup is recorded without a verified `<version>:<dirty>` revision |
| restore fails with `declares an unverifiable schema revision` | `meta.json` `schema_revision` is `unknown`/`uninitialized`/empty (tampered or foreign metadata) | verify the metadata object (§7.1); pick a valid datedir — an unverifiable revision never passes the restore gate |
| restore fails with `re-downloaded archive checksum does not match` | corrupted remote copy | rerun; if persistent, investigate the object store (see §7.1) |
| restore fails with `does not match the checksum sidecar` / `does not match metadata checksum_sha256` | archive or sidecar corrupted in transit or by retention | fetch the object, sidecar and metadata manually and verify (§7.1); pick a different datedir |
| restore fails with `checksum sidecar is not in '<sha256>  <filename>' format` or `does not match object key basename` | tampered or foreign sidecar paired with the archive | investigate the object store; the archive is not trusted without a valid, matching sidecar |
| restore fails `refusing to restore into the live production postgres container/endpoint` | `--restore-pg` resolved to the production postgres | supply a distinct disposable restore container/server (see §5) |
| restore fails with `attached to the production postgres network` | the drill container is attached to a network the production postgres is also on | re-run the drill container on its own dedicated drill network (see §5) |
| restore fails with `using host networking` / `NetworkMode=none` / `attached to the ... network driver` | the drill container uses the host/none network drivers, or a declared `host`/`none` restore-network | re-run the drill container on a dedicated non-host docker network (see §5); a host-network target is never provably isolated |
| restore fails with `does not declare bodysense.restore-network` / `not attached to its declared bodysense.restore-network` | the dedicated drill network is not declared on the container or not actually attached | declare `--label bodysense.restore-network=<drill network>` at creation and attach the container to that network (see §5) |
| restore fails with `attached to networks beyond its declared drill network` | the drill container is on another network (e.g. a bridge network that also exists on the host) besides its declared drill network and the production network | re-create the drill container attached to its declared drill network only (one `--network`, no additional `docker network connect`) (see §5) |
| restore fails with `publishes host ports` | the drill container maps a host port (`-p 5432:5432`), making it reachable from the host | re-create it without any `-p`/`--publish` (see §5); an internode-only drill needs no host port |
| restore fails with `unable to inspect the ... container network(s)` | docker inspect of the production or restore container's network set failed/parsed incompletely | check the docker daemon and container state; the restore is refused because isolation cannot be proven — never treated as isolated |
| restore fails with `does not declare bodysense.restore-project=...` or `refusing a non-disposable target` | the drill container lacks the disposable labels | re-create it with `--label bodysense.restore-project=<target-project>` and `--label bodysense.disposable-restore=yes` |
| restore fails with `is not running` | the drill container is stopped | start/restart the drill container |
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