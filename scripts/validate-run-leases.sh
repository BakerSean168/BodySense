#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
IMAGE="${RUNTIME_TEST_POSTGRES_IMAGE:-pgvector/pgvector:pg18}"
CONTAINER="bodysense-run-lease-$$"
DB="bodysense_run_lease_test"
PASSWORD="run-lease-test"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$CONTAINER" \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB="$DB" \
  "$IMAGE" >/dev/null
for _ in $(seq 1 40); do
  docker exec "$CONTAINER" pg_isready -U postgres -d "$DB" >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$CONTAINER" pg_isready -U postgres -d "$DB" >/dev/null

for migration in $(find "$ROOT/apps/api/migrations" -maxdepth 1 -type f -name '*.up.sql' | sort -V); do
  docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null < "$migration"
done

IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CONTAINER")"
(
  cd "$ROOT/apps/api"
  BODYSENSE_INTEGRATION_DATABASE_URL="host=$IP user=postgres password=$PASSWORD dbname=$DB port=5432 sslmode=disable" \
    go test ./internal/repository -run '^TestRunLease.*Postgres$' -count=1 -v
)

echo 'RUN_LEASE_INTEGRATION=PASS'
