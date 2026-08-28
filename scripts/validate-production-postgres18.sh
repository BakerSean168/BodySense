#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

reject_match() {
  local pattern="$1" file="$2"
  if grep -q "$pattern" "$file"; then
    echo "unexpected legacy PostgreSQL reference pattern=$pattern file=$file" >&2
    exit 1
  fi
}

reject_path() {
  local path="$1"
  if [ -e "$path" ]; then
    echo "unexpected legacy PostgreSQL path: $path" >&2
    exit 1
  fi
}

bash -n scripts/production-postgres18-reset.sh scripts/production-deploy-watch.sh scripts/setup-server.sh scripts/setup-postgres18-client-wrappers.sh

grep -q '^POSTGRES_MAJOR=18$' .env.production
grep -q '^POSTGRES_DATA_VOLUME=bodysense-postgres-pg18$' .env.production
grep -q 'pgvector:pg18' docker/docker-compose.prod.yml
reject_match 'pgvector:pg16' docker/docker-compose.prod.yml
grep -q 'postgres-prod-data-pg18:/var/lib/postgresql$' docker/docker-compose.prod.yml
grep -Fq "name: \${POSTGRES_DATA_VOLUME:-bodysense-postgres-pg18}" docker/docker-compose.prod.yml

grep -q 'pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a' .github/workflows/mirror-production-infra.yml
grep -q 'target: pgvector:pg18' .github/workflows/mirror-production-infra.yml
reject_match 'source: pgvector/pgvector:pg16' .github/workflows/mirror-production-infra.yml

grep -q 'name: PostgreSQL 18 current-history replay' .github/workflows/ci.yml
grep -q 'name: PostgreSQL 18 production-baseline upgrade' .github/workflows/ci.yml
grep -q 'Prepare PostgreSQL 18 client toolchain' .github/workflows/ci.yml
grep -q 'setup-postgres18-client-wrappers.sh' .github/workflows/ci.yml
grep -q '^FROM alpine:3.24$' apps/api/Dockerfile
grep -q 'postgresql18-client' apps/api/Dockerfile
reject_match 'postgresql16-client' apps/api/Dockerfile
[ "$(grep -c 'image: pgvector/pgvector:pg18' .github/workflows/ci.yml)" -ge 3 ]
reject_match 'image: pgvector/pgvector:pg16' .github/workflows/ci.yml
[ -s apps/api/migrations/baselines/production-v29.sql ]
grep -q 'Dumped from database version 18' apps/api/migrations/baselines/production-v29.sql
reject_path apps/api/migrations/baselines/production-pg16-v29.sql

grep -q 'production-postgres18-reset.sh /runtime/scripts/production-postgres18-reset.sh' docker/Dockerfile.runtime
grep -q 'production-postgres18-reset.sh" cutover' scripts/production-deploy-watch.sh
grep -q 'production-postgres18-reset.sh" commit' scripts/production-deploy-watch.sh
grep -q 'fresh PostgreSQL 18 detected; bootstrapping API migrations before AI service' scripts/production-deploy-watch.sh
python3 - <<'PY_ORDER'
from pathlib import Path
s = Path('scripts/production-deploy-watch.sh').read_text()
start = s.index('if [ "$reset_status" = cutover_complete ]; then')
end = s.index('compose up -d --no-deps web', start)
block = s[start:end]
then_block, else_block = block.split('else', 1)
assert then_block.index('deploy_api_service') < then_block.index('deploy_ai_service'), then_block
assert else_block.index('deploy_ai_service') < else_block.index('deploy_api_service'), else_block
PY_ORDER
grep -q 'production-postgres18-reset.sh' scripts/setup-server.sh
reject_path scripts/production-postgres-major-upgrade.sh
reject_path scripts/validate-production-postgres-major-upgrade.sh
reject_match 'validate-production-postgres-major-upgrade.sh' scripts/validate-repo.sh

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
