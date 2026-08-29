# BodySense deployment architecture

> Current deployment architecture, 2026-08-28.

## Environment roles

- **GCP-dev (`gcp-dev-01`)** is the primary development, ephemeral validation, persistent staging and production-operations control host.
- **Oracle2** is detached from BodySense application development/deployment and must not receive new BodySense runtimes.
- **GitHub Actions** is the CI and release build plane.
- **Alibaba Cloud ACR** is the production image registry.
- **Alibaba Cloud ECS** (`body.bakersean.top`) is the only production runtime.
- **DigitalOcean** is retired and preserved only under `docs/archive/deployment/digitalocean/`.

### GCP development and staging lanes

GCP uses the canonical project block `20100-20199` from `my-infrastructure/projects/bodysense.yaml`:

| Lane       | Runtime                             | Host ports                                                                               | Lifecycle               |
| ---------- | ----------------------------------- | ---------------------------------------------------------------------------------------- | ----------------------- |
| direct dev | host Web/API/AI + Docker-only infra | Web `20100`, API `20101`, AI `20102`, PostgreSQL `20110`, Redis `20111`, LiteLLM `20112` | development session     |
| validator  | isolated Docker Compose             | dynamically allocated loopback ports                                                     | ephemeral, auto-cleaned |
| staging    | production-shaped Docker Compose    | only Web/nginx ingress `127.0.0.1:20150`                                                 | persistent              |
| production | Alibaba ECS Docker Compose          | public `80/443` through Caddy                                                            | persistent              |

`./scripts/dev-runtime.sh` owns direct-dev startup order: infrastructure health -> Go API migration/schema bootstrap -> Python AI health -> Vite Web. `./scripts/staging-runtime.sh` owns the persistent `bodysense-staging` Compose project. Development, staging, CI and steady-state production use PostgreSQL 18. Historical production schema upgrades are exercised by a PG18 production-baseline scenario rather than by retaining compatibility with the PostgreSQL 16 engine.

GCP application host ports bind loopback. Human remote access is provided by Tailscale Serve/TLS; database, Redis, LiteLLM and AI internal staging ports are not exposed directly.

### Public immutable asset plane

Application traffic and immutable byte delivery intentionally use different network boundaries. Authenticated HTML/API/SSE and all user-varying health data remain on the application origin. Public, user-independent Vite hashes and anatomy assets may be distributed through the BodySense static CDN.

```text
Browser
  ├─ application origin
  │    ├─ index.html
  │    ├─ /api/*
  │    ├─ auth
  │    └─ consultation SSE / health data
  └─ https://assets.bakersean.top
       ├─ /web/<git-revision>/assets/*
       └─ /anatomy/vanatome/1.4.0/*
```

The CDN namespace is immutable: Web assets are keyed by the full source revision and anatomy assets by the pinned atlas release. The production workflow publishes and verifies CDN bytes before building/promoting the Web artifact that references them, then performs a Web-image-to-CDN coherence gate. No user, conversation, diagnosis or BodyState identifiers may appear in static asset paths.

When `STATIC_ASSET_CDN_BASE` is not configured, the Web image keeps same-origin behavior so local development and emergency rollback do not depend on R2 credentials. See `docs/runbooks/static-asset-cdn.md`.

## Production delivery flow

```text
feature branch
    -> PR
    -> CI
       - repository quality gate
       - migration history immutability
       - PostgreSQL 18 current-history + production-baseline validation
       - browser longitudinal E2E
    -> merge main
    -> full main CI must finish successfully
    -> release-please
    -> release PR + CI
    -> vX.Y.Z tag / GitHub Release
    -> Build & Publish Production Images
       - build Web/API/AI/runtime artifacts in parallel
       - push immutable vX.Y.Z images
       - only after all four succeed: promote all to prod-latest
    -> Alibaba Cloud systemd deploy watcher
       - pull the three prod-latest pointers
       - require identical OCI revision labels across Web/API/AI/runtime
       - extract runtime files from the coherent ACR runtime image
       - validate Compose against production secrets
       - back up PostgreSQL
       - for the one-time production transition, discard the legacy database and start a fresh PG18 volume before application migrations
       - deploy AI -> API -> Web with health gates
       - commit the PG18 database boundary before re-exposing Caddy
       - verify public HTTPS health
```

The critical invariant is:

> `prod-latest` is only a movable pointer. A deployment is eligible only when Web, API, AI Service and the runtime bundle all point to artifacts built from the same immutable Git revision.

This prevents a polling deployer from serving a mixed release while ACR promotion is still in progress.

## CI

`.github/workflows/ci.yml` runs for pushes and pull requests to the single long-lived trunk, `main`. Short-lived feature/fix/refactor branches are validated through PRs targeting `main`; staging is an artifact/runtime channel rather than a permanent Git branch. Delivery Platform V3 target semantics are defined in [`delivery-platform-v3.md`](./delivery-platform-v3.md).

### Repository quality gate

The same `scripts/validate-repo.sh` entrypoint is used locally and in CI. It covers lint, typecheck, Go/Python/Web tests, Agent qualification/promotion evals, LiteLLM smoke and production builds.

Third-party GitHub Actions are pinned to immutable commit SHAs rather than movable major-version tags. `.github/dependabot.yml` tracks the `github-actions` ecosystem weekly so upgrades arrive as reviewable PRs instead of silently changing the CI/CD execution environment.

`scripts/validate-migration-history.sh` additionally enforces migration history rules:

- every non-legacy migration version has both `up` and `down` files;
- published migration 29 can never disappear again;
- every currently published SQL migration is frozen by SHA-256 manifest;
- adding a migration requires explicitly extending the checksum manifest;
- editing a published migration fails CI.

Legacy migration-number gaps `2`, `3`, `5` predate the current production baseline and are explicitly grandfathered. From the known published production baseline onward, the sequence is continuous.

### Migration validation

Migration CI has two intentionally different **scenarios**, both on the single supported PostgreSQL / pgvector 18 runtime:

- `PostgreSQL 18 current-history replay` rebuilds the current migration history, stops at the published baseline `29`, then validates `29 -> latest` and latest `down -> up`.
- `PostgreSQL 18 production-baseline upgrade` restores the PG18-normalized `production-v29.sql` schema fixture captured from the historical production-v29 shape, then validates `29 -> latest`, domain semantics and the PG18 `pg_dump` / `pg_restore` recovery path.

The second job protects **old production schema/data upgrade semantics**, not compatibility with an old PostgreSQL engine. Development, staging, CI and steady-state production all use PostgreSQL 18.

This baseline exists because production was found at migration 29 while migration 29 had been deleted from the repository; that deletion caused the v0.4.0 API container to restart-loop. Published migrations are now treated as immutable release artifacts.

## Repository governance

`main` is intended to be protected by required PR flow and the five CI checks (`Repository quality gate`, both PostgreSQL migration jobs, Browser E2E and `commit-lint`), with administrator bypass disabled. The `production` GitHub Environment is restricted to `main` and `v*` refs and stores the ACR credential set as environment secrets. `.github/workflows/repository-governance.yml` is the idempotent admin workflow that applies these repository settings using the existing `BODYSENSE_WORKFLOW` PAT without exposing its value.

## Release management

`release-please` owns application versions and GitHub releases. It is triggered only by a successful completed `CI` run for `main`, and it first verifies that the tested revision is still the current `main` HEAD. Conventional commits update the release PR; merging that PR causes another full main CI run, and only after that run succeeds can release-please create the `vX.Y.Z` tag and GitHub Release.

`.github/workflows/docker-deploy.yml` consumes the release tag. Before any ACR login/build, it verifies that the exact tagged revision is reachable from `main` and has a completed successful `CI` push run. Manual production promotion is restricted to `main`.

### Immutable build first

Web, API, AI Service and a small runtime configuration bundle are built in parallel as Linux/amd64-compatible OCI images and pushed only with the immutable release tag first:

```text
bodysense-web:vX.Y.Z
bodysense-api:vX.Y.Z
bodysense-ai-service:vX.Y.Z
bodysense-runtime:vX.Y.Z
```

The runtime image contains only tracked non-secret production runtime files (`.env.production`, Compose, Caddy, LiteLLM config, the deploy watcher, the off-host backup/restore scripts and the off-host systemd units). Secrets remain on the production host and are never embedded in the image.

Each image records `org.opencontainers.image.revision=<git SHA>`.

ACR does not accept the Buildx provenance manifest class used by default, so the workflow explicitly uses `provenance: false`.

### Promotion second

The promotion job is attached to the GitHub `production` Environment, which gives production pointer changes a dedicated deployment history and a place for repository-admin protection rules. Only after all three immutable builds succeed does the promotion job move:

```text
bodysense-web:prod-latest
bodysense-api:prod-latest
bodysense-ai-service:prod-latest
bodysense-runtime:prod-latest
```

to the new immutable artifacts. If one build fails, **none** of the production pointers are promoted.

## Production deploy watcher

The previous `containrrr/watchtower` deployment path is retired. It updated containers independently and had no knowledge of application release identity, migration safety or runtime-file compatibility.

The production host now uses:

- `scripts/production-deploy-watch.sh`
- `deploy/systemd/bodysense-deploy-watch.service`
- `deploy/systemd/bodysense-deploy-watch.timer`

The timer polls every ~2 minutes. The script uses `flock`, so overlapping deployment attempts cannot race.

### Eligibility

Before touching running containers it:

1. pulls Web/API/AI `prod-latest`;
2. reads each image OCI revision label;
3. refuses deployment unless all three revisions are non-empty and identical;
4. pulls the `bodysense-runtime:prod-latest` artifact and requires its OCI revision label to match Web/API/AI;
5. extracts `.env.production`, production Compose, Caddy, LiteLLM config, the deployment watcher, the off-host backup/restore scripts and the off-host systemd units from that runtime artifact;
6. validates `docker compose config` with the server's untracked `.env.production.local`.

Secrets are never fetched from Git and are never overwritten.

### Deployment

For an eligible release it:

1. creates a PostgreSQL custom-format backup plus SHA-256;
2. updates AI Service and waits for health;
3. updates API and waits for health (API applies pending migrations on startup);
4. updates Web and waits for health;
5. reloads Caddy configuration;
6. checks `https://body.bakersean.top/api/health`;
7. records the deployed Git revision in `/opt/bodysense/.deploy-state`.

Production backups generated by the watcher are retained for 14 days by default.

### Failure handling and safe rollback

Before mutating runtime files or containers, the watcher captures the current application/LiteLLM image IDs, records the current `schema_migrations` state and creates the PostgreSQL backup. The custom-format dump is copied into the running PostgreSQL container and must pass `pg_restore --list` archive validation before deployment is allowed to continue. The previous runtime bundle is archived before the new bundle is installed.

If a deployment later fails, the watcher compares the database schema state with the pre-deploy value:

- if the schema is unchanged and the previous image/runtime set is complete, it automatically restores the previous runtime bundle and locally tagged previous images, waits for health, and verifies the public API;
- if the schema changed or cannot be verified, it deliberately does **not** perform a blind application rollback. The failed revision is blocked and the database backup is retained for operator recovery.

`.deploy-blocked` records the failed revision, rollback outcome, schema state before/after, and the backup filename. This makes rollback conservative around migrations: automatic rollback is only allowed when the watcher can prove the database contract did not move. Schema changes should therefore follow expand/migrate/contract compatibility rules rather than relying on destructive down-migrations during a failed deploy.

Runtime bundle archives are also retained for 14 days.

## Production infrastructure image mirrors

Infrastructure images are mirrored into Alibaba ACR by `.github/workflows/mirror-production-infra.yml`. Every upstream source is pinned by OCI digest, so rerunning the mirror cannot silently consume a newer image behind a mutable upstream tag. Normal production deploys therefore do not depend on Docker Hub or the upstream LiteLLM registry. This workflow is manual because infrastructure versions change deliberately, not on every application release.

Current pinned mirrors: PostgreSQL/pgvector 18, Redis 7 Alpine, Caddy 2 Alpine, and LiteLLM v1.97.0. Their human-readable tags remain stable inside ACR, while the mirror workflow fixes the exact upstream OCI digest in Git.

## Production database

BodySense supports PostgreSQL 18 + pgvector as the single steady-state database runtime. The 2026-08-28 production transition discards the non-authoritative legacy database and starts a fresh PostgreSQL 18 volume through the release watcher. The old data directory is never reused as a PG18 data directory and its legacy volume is deleted after PG18 plus the application stack pass health gates.

CI keeps separate current-history and production-baseline upgrade scenarios, but both run PostgreSQL 18. This preserves schema/data upgrade coverage without carrying PostgreSQL 16 engine compatibility.

## Off-host PostgreSQL backup and restore (BS-PROD-012)

In addition to the deploy watcher's same-host backups, production keeps an
**operator-owned off-host** copy of the PostgreSQL database on a private
OSS/S3-compatible destination (Alibaba Cloud OSS `cn-hangzhou`):

- `scripts/production-offhost-backup.sh --backup` runs daily via
  `bodysense-offhost-backup.timer` (`OnCalendar=*-*-* 02:10:00 Asia/Shanghai` —
  the timezone is embedded in the calendar expression so the schedule does not
  depend on the host timezone). It produces a
  custom-format dump through the normal network protocol
  (`docker compose exec postgres pg_dump -Fc`), records its SHA-256 as a sidecar
  object and a metadata object (schema revision, checksum, source, retention),
  uploads the trio to `OFFHOST_BACKUP_BUCKET` under `OFFHOST_BACKUP_PREFIX`,
  re-downloads the checksum object for an end-to-end round-trip, and prunes
  objects older than `OFFHOST_BACKUP_RETENTION_DAYS` (the newest day directory is
  never pruned). Retention is apply-or-fail: if the off-host object listing that
  drives pruning cannot be fetched, the backup aborts and `last-success.json` is
  not recorded, so an unbounded retention window can never coexist with a
  healthy freshness state.
- `scripts/production-offhost-backup.sh --check-freshness` runs hourly via
  `bodysense-offhost-freshness.timer` (`OnCalendar=*-*-* *:00:00 Asia/Shanghai`,
  also embedded in the expression).
  It reads the local `last-success.json`
  state and, when `OFFHOST_BACKUP_FRESHNESS_PROBE=object`, confirms the latest
  archive still exists remotely. Freshness is compared in whole seconds (a
  backup is stale once `now - last_success` exceeds the threshold, with no
  whole-hour truncation) and a future-dated last-success is rejected, never
  treated as fresh. A missing/stale state file exits non-zero, emits
  `OFFHOST_BACKUP_FRESH=FAIL reason=...` and optionally runs the configured
  `OFFHOST_BACKUP_ALERT_CMD`.
- `scripts/restore-production-backup.sh` is the operator-only restore drill. It
  never restores into the production database or production postgres server.
  The restore target must be an explicitly supplied disposable PostgreSQL
  container (`--restore-pg container:<id|name>`), and before anything else the
  operator proves that container is isolated from production via `docker
inspect` — fail-closed, refusing on: container-ID equality with the production
  postgres container, membership in the production Compose project, ANY Docker
  network shared with the production postgres container, attachment to any
  network beyond the container's declared `bodysense.restore-network` (the drill
  network must be the container's sole network), any published host port
  (`HostConfig.PortBindings`), a non-running state, or a missing/incorrect
  declaration of `bodysense.restore-project=<target-project>`
  and `bodysense.disposable-restore=yes` (drill containers must therefore run on
  their own dedicated drill network, never on the production postgres network,
  publishing no host ports). The declared drill network itself must contain no
  production container: every member of `bodysense.restore-network` is
  enumerated and any member that is the production postgres container or carries
  the production `com.docker.compose.project` label refuses the drill (Docker
  bridge connectivity is bidirectional, so nowhere — including an api container
  joined to the drill network to host validators — may a production container
  share the drill network with the disposable restore database).
  All `psql`/`pg_restore` and `docker cp`
  operations target that disposable server exclusively. The target must differ
  from `DB_NAME`, the project must differ from `bodysense`, the target database
  must not already exist, and `--confirm-target-isolated=yes` is mandatory. It
  verifies the SHA-256 sidecar strictly (syntax, attested filename, digest
  equality with the metadata `checksum_sha256`) and proves the downloaded
  archive matches the sidecar and the metadata, validates the archive with
  `pg_restore --list`, restores into the fresh disposable database on the
  disposable server with `--no-owner --no-privileges`, verifies the restored
  schema revision equals the backup metadata, and runs the `domain-validator`
  and `migration-validator` binaries (built into the API image at
  `/app/validators/`) against the restored database inside a **separate
  disposable validator container** (`docker run --rm`) taken from the api
  container's `Config.Image` (or the pinned `OFFHOST_VALIDATOR_IMAGE`) and
  attached only to the drill network — the production api container is never
  joined to the drill network. The database password
  reaches the validators only through `PGPASSWORD` in the process environment
  (injected via an `--env-file` on the `docker run` path), never through
  `-database-url` or any process command line.
- Object-store credentials are host-only least-privilege keys in
  `.env.production.local` (`OFFHOST_BACKUP_ACCESS_KEY` /
  `OFFHOST_BACKUP_SECRET_KEY`, limited to GetObject/PutObject/DeleteObject/ListBucket
  on the backup bucket). They are never written into artifacts or Git and are
  supplied to `scripts/offhost-s3.py` only through the process environment —
  the client refuses command-line credential arguments so secrets cannot leak
  through `/proc/*/cmdline` or process listings.
- Off-host backups are retained 30 days by default (independent of the watcher's
  same-host 14-day retention).

See [docs/security/offhost-backup-restore-runbook.md](../security/offhost-backup-restore-runbook.md).

## Production Compose and model gateway

`docker/docker-compose.prod.yml` preserves the North-Star model boundary: AI Service receives only `LITELLM_BASE_URL` / `LITELLM_API_KEY`; physical provider credentials are injected only into the standalone LiteLLM gateway.

The CI/CD repair does **not** reintroduce direct provider routing. The production reconciliation is limited to real infrastructure facts (PostgreSQL 18, Alibaba ACR mirrors) and the safer deploy watcher. `LITELLM_MASTER_KEY` is an internal gateway credential stored only in `.env.production.local`; `MIMO_API_KEY` may be empty while an available OpenRouter fallback is configured.

## Secret boundaries

Tracked:

- `.env.production` — non-sensitive production settings
- `docker/docker-compose.prod.yml`
- `docker/Caddyfile`
- deployment watcher/systemd definitions
- off-host backup/restore scripts and their systemd units

Untracked on production:

- `.env.production.local` — database/password/API keys
- `/opt/bodysense/backups`, runtime backups and deploy state — persistent production state

The bootstrap repository checkout is kept separately at `/opt/bodysense-source` by default. `scripts/setup-server.sh` may reset that disposable checkout, but it never removes `/opt/bodysense`, so rerunning bootstrap cannot erase secrets, backups or deployment state.

Untracked on Oracle2:

- `.secrets/` — production SSH material

`.secrets/`, private key formats and common SSH private-key names are gitignored.

## Operational commands

Production watcher status:

```bash
systemctl status bodysense-deploy-watch.timer
systemctl status bodysense-deploy-watch.service
journalctl -u bodysense-deploy-watch.service -n 100
```

Manual coherent-release check/deploy:

```bash
/opt/bodysense/scripts/production-deploy-watch.sh --force
```

Off-host backup status:

```bash
systemctl status bodysense-offhost-backup.timer
systemctl status bodysense-offhost-freshness.timer
journalctl -u bodysense-offhost-backup.service -n 100
journalctl -u bodysense-offhost-freshness.service -n 100
/opt/bodysense/scripts/production-offhost-backup.sh --check-freshness
```

Production stack:

```bash
cd /opt/bodysense
docker compose -p docker -f docker/docker-compose.prod.yml \
  --env-file .env.production \
  --env-file .env.production.local ps
```

### Runtime artifact boundary

Normal production reconciliation is ACR-contained: once a server is bootstrapped, a release does not need GitHub to fetch runtime files. GitHub remains the source plane for CI/release creation and for first-host bootstrap; ACR is the complete release artifact plane used by the production watcher.
