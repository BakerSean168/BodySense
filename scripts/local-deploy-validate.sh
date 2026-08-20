#!/usr/bin/env bash
set -euo pipefail
export BODYSENSE_E2E_STUB_AI=1

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

project="bodysense-validator"
export COMPOSE_PROJECT_NAME="$project"

pick_port() {
  python3 - "$1" <<'PY'
import socket
import sys

preferred = int(sys.argv[1])
for candidate in (preferred, 0):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        try:
            sock.bind(("127.0.0.1", candidate))
        except OSError:
            continue
        print(sock.getsockname()[1])
        break
else:
    raise SystemExit("unable to allocate validator port")
PY
}

export DB_PORT="${VALIDATOR_DB_PORT:-$(pick_port 55432)}"
export REDIS_PORT="${VALIDATOR_REDIS_PORT:-$(pick_port 56379)}"
export LITELLM_PORT="${VALIDATOR_LITELLM_PORT:-$(pick_port 54000)}"
export API_PORT="${VALIDATOR_API_PORT:-$(pick_port 18080)}"
export AI_SERVICE_PORT="${VALIDATOR_AI_PORT:-$(pick_port 18100)}"
export WEB_PORT="${VALIDATOR_WEB_PORT:-$(pick_port 15173)}"
export DB_USER="bodysense"
export DB_PASSWORD="bodysense123"
export DB_NAME="bodysense"
export REDIS_PASSWORD="bodysense123"
export JWT_SECRET_KEY="bodysense-local-validator-secret"
# The validator must be hermetic: exercise real Agent/runtime contracts without
# relying on external model availability or credentials.
export BODYSENSE_DETERMINISTIC_AI="true"
compose=(docker compose -f docker/docker-compose.yml --profile dev)

cleanup() {
  if [[ "${KEEP_VALIDATOR_STACK:-0}" != "1" ]]; then
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_http() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "$name=PASS"
      return 0
    fi
    sleep 2
  done
  echo "$name=FAIL" >&2
  return 1
}

if [[ "${SKIP_QUALITY:-0}" != "1" ]]; then
  bash scripts/validate-repo.sh
fi

"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${compose[@]}" build api ai-service web
"${compose[@]}" up -d postgres-dev redis-dev ai-service api web

wait_http "http://127.0.0.1:${API_PORT}/api/health" "API_HEALTH"
wait_http "http://127.0.0.1:${AI_SERVICE_PORT}/health" "AI_HEALTH"
wait_http "http://127.0.0.1:${WEB_PORT}" "WEB_HEALTH"

migration_db="bodysense_migration_validator"
"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS ${migration_db} WITH (FORCE);" >/dev/null
"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${migration_db};" >/dev/null
(
  cd apps/api
  go run ./cmd/migration-validator \
    -database-url "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${migration_db}?sslmode=disable" \
    -migrations "file://migrations"
)
(
  cd apps/api
  go run ./cmd/domain-validator \
    -database-url "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${migration_db}?sslmode=disable"
)

E2E_BASE_URL="http://127.0.0.1:${WEB_PORT}" \
E2E_API_BASE_URL="http://127.0.0.1:${API_PORT}" \
pnpm e2e

echo "LOCAL_DEPLOY_VALIDATION=PASS"
