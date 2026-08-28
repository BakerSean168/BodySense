#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

bash -n scripts/production-postgres-major-upgrade.sh scripts/production-deploy-watch.sh scripts/setup-server.sh

grep -q '^POSTGRES_MAJOR=18$' .env.production
grep -q '^POSTGRES_DATA_VOLUME=bodysense-postgres-pg18$' .env.production
grep -q 'pgvector:pg18' docker/docker-compose.prod.yml
! grep -q 'pgvector:pg16' docker/docker-compose.prod.yml
grep -q 'postgres-prod-data-pg18:/var/lib/postgresql$' docker/docker-compose.prod.yml
grep -q 'name: ${POSTGRES_DATA_VOLUME:-bodysense-postgres-pg18}' docker/docker-compose.prod.yml

grep -q 'pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a' .github/workflows/mirror-production-infra.yml
grep -q 'target: pgvector:pg18' .github/workflows/mirror-production-infra.yml
! grep -q 'source: pgvector/pgvector:pg16' .github/workflows/mirror-production-infra.yml

grep -q 'name: PostgreSQL 18 current-history replay' .github/workflows/ci.yml
grep -q 'name: PostgreSQL 18 production-baseline upgrade' .github/workflows/ci.yml
grep -q 'Prepare PostgreSQL 18 client toolchain' .github/workflows/ci.yml
grep -q 'setup-postgres18-client-wrappers.sh' .github/workflows/ci.yml
grep -q '^FROM alpine:3.24$' apps/api/Dockerfile
grep -q 'postgresql18-client' apps/api/Dockerfile
! grep -q 'postgresql16-client' apps/api/Dockerfile
[ "$(grep -c 'image: pgvector/pgvector:pg18' .github/workflows/ci.yml)" -ge 3 ]
! grep -q 'image: pgvector/pgvector:pg16' .github/workflows/ci.yml
[ -s apps/api/migrations/baselines/production-v29.sql ]
grep -q 'Dumped from database version 18' apps/api/migrations/baselines/production-v29.sql
! test -e apps/api/migrations/baselines/production-pg16-v29.sql

grep -q 'production-postgres-major-upgrade.sh /runtime/scripts/production-postgres-major-upgrade.sh' docker/Dockerfile.runtime
grep -q 'runtime declares POSTGRES_MAJOR but bundle is missing PostgreSQL major-upgrade operator' scripts/production-deploy-watch.sh
grep -q 'production-postgres-major-upgrade.sh" cutover' scripts/production-deploy-watch.sh
grep -q 'production-postgres-major-upgrade.sh" commit' scripts/production-deploy-watch.sh
grep -q 'rollback_postgres_major_upgrade' scripts/production-deploy-watch.sh
grep -q 'PostgreSQL major upgrade is committed; preserving PG18 runtime' scripts/production-deploy-watch.sh
grep -q 'validate-production-postgres-major-upgrade.sh' scripts/validate-repo.sh
grep -q 'use the release watcher major-upgrade transaction instead of setup-server' scripts/setup-server.sh

secret_env=$(mktemp)
trap 'rm -f "$secret_env"' EXIT
cat > "$secret_env" <<'ENV'
DB_PASSWORD=postgres18-validation
REDIS_PASSWORD=postgres18-validation
JWT_SECRET_KEY=postgres18-validation
LITELLM_MASTER_KEY=postgres18-validation
REGISTRY=example.invalid
ENV
docker compose -f docker/docker-compose.prod.yml --env-file .env.production --env-file "$secret_env" config -q

echo POSTGRES18_PRODUCTION_CONTRACT=PASS
