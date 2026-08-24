#!/usr/bin/env bash
set -euo pipefail

: "${COMPOSE_PROJECT_NAME:?COMPOSE_PROJECT_NAME is required}"
: "${DB_USER:?DB_USER is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${API_PORT:?API_PORT is required}"

compose=(docker compose -f docker/docker-compose.yml --profile dev -p "$COMPOSE_PROJECT_NAME")

# This helper is intentionally local-E2E only. It simulates a process death
# after lease expiry so the normal startup reconciler, not a test-only API,
# produces the durable execution_lost terminal event.
"${compose[@]}" exec -T postgres-dev psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" -c \
  "UPDATE runs SET lease_expires_at = now() - interval '1 minute' WHERE status = 'running';" >/dev/null
"${compose[@]}" restart api >/dev/null

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${API_PORT}/api/health" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 1
done

echo "API did not recover after E2E restart" >&2
exit 1
