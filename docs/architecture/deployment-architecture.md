# BodySense deployment architecture

> Current production architecture, 2026-08-22.

## Environment roles

- **Oracle2** is development / local deploy validation / temporary product preview. It is not production.
- **GitHub Actions** is the CI and release build plane.
- **Alibaba Cloud ACR** is the production image registry.
- **Alibaba Cloud ECS** (`body.bakersean.top`) is the only production runtime.
- **DigitalOcean** is retired and preserved only under `docs/archive/deployment/digitalocean/`.

## Production delivery flow

```text
feature branch
    -> PR
    -> CI
       - repository quality gate
       - migration history immutability
       - PostgreSQL 16 + 18 baseline/replay validation
       - browser longitudinal E2E
    -> merge main
    -> full main CI must finish successfully
    -> release-please
    -> release PR + CI
    -> vX.Y.Z tag / GitHub Release
    -> Build & Publish Production Images
       - build Web/API/AI in parallel
       - push immutable vX.Y.Z images
       - only after all three succeed: promote all to prod-latest
    -> Alibaba Cloud systemd deploy watcher
       - pull the three prod-latest pointers
       - require identical OCI revision labels
       - fetch the exact repository revision for runtime files
       - validate Compose against production secrets
       - back up PostgreSQL
       - deploy AI -> API -> Web with health gates
       - verify public HTTPS health
```

The critical invariant is:

> `prod-latest` is only a movable pointer. A deployment is eligible only when Web, API and AI Service all point to images built from the same immutable Git revision.

This prevents a polling deployer from serving a mixed release while ACR promotion is still in progress.

## CI

`.github/workflows/ci.yml` runs for pushes and pull requests to `main` / `dev`.

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

Migration CI has two intentionally different paths:

- PostgreSQL / pgvector 18 rebuilds the current migration history, stops at the published baseline `29`, then validates `29 -> latest` and latest `down -> up`.
- PostgreSQL / pgvector 16 restores the schema-only `production-pg16-v29.sql` fixture captured from the real production-v29 schema, then validates `29 -> latest` and domain semantics. It also creates a custom-format `pg_dump`, validates the archive, restores it into a fresh database, checks the migration version and reruns the domain validator against the restored database. This avoids pretending the modern PostgreSQL-18 migration history can recreate an old PG16 database from version 1 while continuously exercising the backup/restore path used by production.

This exists because production was found at migration 29 while migration 29 had been deleted from the repository; that deletion caused the v0.4.0 API container to restart-loop. Published migrations are now treated as immutable release artifacts.

## Release management

`release-please` owns application versions and GitHub releases. It is triggered only by a successful completed `CI` run for `main`, and it first verifies that the tested revision is still the current `main` HEAD. Conventional commits update the release PR; merging that PR causes another full main CI run, and only after that run succeeds can release-please create the `vX.Y.Z` tag and GitHub Release.

`.github/workflows/docker-deploy.yml` consumes the release tag. Before any ACR login/build, it verifies that the exact tagged revision is reachable from `main` and has a completed successful `CI` push run. Manual production promotion is restricted to `main`.

### Immutable build first

Web, API and AI Service are built in parallel as Linux/amd64 images and pushed only with the immutable release tag first:

```text
bodysense-web:vX.Y.Z
bodysense-api:vX.Y.Z
bodysense-ai-service:vX.Y.Z
```

Each image records `org.opencontainers.image.revision=<git SHA>`.

ACR does not accept the Buildx provenance manifest class used by default, so the workflow explicitly uses `provenance: false`.

### Promotion second

The promotion job is attached to the GitHub `production` Environment, which gives production pointer changes a dedicated deployment history and a place for repository-admin protection rules. Only after all three immutable builds succeed does the promotion job move:

```text
bodysense-web:prod-latest
bodysense-api:prod-latest
bodysense-ai-service:prod-latest
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
4. fetches that exact Git revision from the public BodySense repository;
5. stages `.env.production`, production Compose, Caddy and deployment scripts from the same revision;
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

Before mutating runtime files or containers, the watcher captures the current application/LiteLLM image IDs, records the current `schema_migrations` state and creates the PostgreSQL backup. The previous runtime bundle is archived before the new bundle is installed.

If a deployment later fails, the watcher compares the database schema state with the pre-deploy value:

- if the schema is unchanged and the previous image/runtime set is complete, it automatically restores the previous runtime bundle and locally tagged previous images, waits for health, and verifies the public API;
- if the schema changed or cannot be verified, it deliberately does **not** perform a blind application rollback. The failed revision is blocked and the database backup is retained for operator recovery.

`.deploy-blocked` records the failed revision, rollback outcome, schema state before/after, and the backup filename. This makes rollback conservative around migrations: automatic rollback is only allowed when the watcher can prove the database contract did not move. Schema changes should therefore follow expand/migrate/contract compatibility rules rather than relying on destructive down-migrations during a failed deploy.

Runtime bundle archives are also retained for 14 days.

## Production infrastructure image mirrors

Infrastructure images are mirrored into Alibaba ACR by `.github/workflows/mirror-production-infra.yml`. Every upstream source is pinned by OCI digest, so rerunning the mirror cannot silently consume a newer image behind a mutable upstream tag. Normal production deploys therefore do not depend on Docker Hub or the upstream LiteLLM registry. This workflow is manual because infrastructure versions change deliberately, not on every application release.

Current pinned mirrors: PostgreSQL/pgvector 16, Redis 7 Alpine, Caddy 2 Alpine, and LiteLLM v1.97.0. Their human-readable tags remain stable inside ACR, while the mirror workflow fixes the exact upstream OCI digest in Git.

## Production database

Current production is PostgreSQL 16 + pgvector. Do **not** replace the production database container with PostgreSQL 18 by changing the image tag in place; a major-version upgrade requires a separate pg_dump/restore or pg_upgrade plan.

Development/validator environments may use PostgreSQL 18, which is why CI validates both versions.

## Production Compose and model gateway

`docker/docker-compose.prod.yml` preserves the North-Star model boundary: AI Service receives only `LITELLM_BASE_URL` / `LITELLM_API_KEY`; physical provider credentials are injected only into the standalone LiteLLM gateway.

The CI/CD repair does **not** reintroduce direct provider routing. The production reconciliation is limited to real infrastructure facts (PostgreSQL 16, Alibaba ACR mirrors) and the safer deploy watcher. `LITELLM_MASTER_KEY` is an internal gateway credential stored only in `.env.production.local`; `MIMO_API_KEY` may be empty while an available OpenRouter fallback is configured.

## Secret boundaries

Tracked:

- `.env.production` — non-sensitive production settings
- `docker/docker-compose.prod.yml`
- `docker/Caddyfile`
- deployment watcher/systemd definitions

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

Production stack:

```bash
cd /opt/bodysense
docker compose -p docker -f docker/docker-compose.prod.yml \
  --env-file .env.production \
  --env-file .env.production.local ps
```
