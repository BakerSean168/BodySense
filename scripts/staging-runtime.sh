#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
ENV_FILE=${BODYSENSE_STAGING_ENV_FILE:-$ROOT/.env.staging.local}
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-bodysense-staging}
export STAGING_BIND_HOST=${STAGING_BIND_HOST:-127.0.0.1}
export STAGING_WEB_PORT=${STAGING_WEB_PORT:-20150}
compose=(docker compose -f docker/docker-compose.staging.yml --env-file "$ENV_FILE")

bootstrap() {
  if [[ -e "$ENV_FILE" ]]; then
    echo "staging env already exists: $ENV_FILE"
    return
  fi
  umask 077
  cat >"$ENV_FILE" <<ENV
DB_USER=bodysense
DB_NAME=bodysense
DB_PASSWORD=$(openssl rand -hex 24)
REDIS_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET_KEY=$(openssl rand -hex 32)
LITELLM_MASTER_KEY=sk-$(openssl rand -hex 24)
EMBEDDING_PROVIDER=hashing
# Optional real provider credentials for non-stub staging inference:
MIMO_API_KEY=
OPENROUTER_API_KEY=
ENV
  echo "created gitignored staging env: $ENV_FILE"
}

require_env() {
  [[ -f "$ENV_FILE" ]] || { echo "missing $ENV_FILE; run: $0 bootstrap" >&2; exit 2; }
}

case "${1:-status}" in
  bootstrap) bootstrap ;;
  up) require_env; "${compose[@]}" up -d --build ;;
  down) require_env; "${compose[@]}" down ;;
  restart) require_env; "${compose[@]}" up -d --build --force-recreate ;;
  status)
    require_env
    "${compose[@]}" ps
    curl -fsS "http://127.0.0.1:${STAGING_WEB_PORT}/api/health" || true
    echo
    ;;
  logs) require_env; "${compose[@]}" logs --tail "${2:-120}" ;;
  *) echo "usage: $0 {bootstrap|up|down|restart|status|logs [lines]}" >&2; exit 2 ;;
esac
