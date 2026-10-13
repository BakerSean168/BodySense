#!/usr/bin/env bash
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
IMAGE="${RUNTIME_TEST_POSTGRES_IMAGE:-pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a}"
CONTAINER="bodysense-body-fact-test-$$"
DB="bodysense_body_fact_test"
PASSWORD="synthetic-body-fact-test"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$CONTAINER" \
  -e POSTGRES_PASSWORD="$PASSWORD" -e POSTGRES_DB="$DB" "$IMAGE" >/dev/null
for _ in $(seq 1 40); do
  docker exec "$CONTAINER" pg_isready -h 127.0.0.1 -U postgres -d "$DB" >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$CONTAINER" pg_isready -h 127.0.0.1 -U postgres -d "$DB" >/dev/null
IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CONTAINER")"
(
  cd "$ROOT/apps/api"
  # Use the same migrator as CI/runtime: separate psql processes hide session
  # settings retained by the pg_dump-derived baseline (notably search_path).
  DATABASE_URL="postgres://postgres:$PASSWORD@$IP:5432/$DB?sslmode=disable"
  go run ./cmd/migration-validator -database-url "$DATABASE_URL" -migrations file://migrations
  BODYSENSE_INTEGRATION_DATABASE_URL="host=$IP user=postgres password=$PASSWORD dbname=$DB port=5432 sslmode=disable" \
    go test -p 2 -race ./internal/repository ./internal/transport/httpapi ./internal/consultation -run '^Test(BodyFact|ThreadProjection|InteractionResume).*Postgres$' -count=1 -v
  # Replay after synthetic test cleanup too; never target a running environment.
  go run ./cmd/migration-validator -database-url "$DATABASE_URL" -migrations file://migrations
)
echo 'BODY_FACT_LEDGER_POSTGRES=PASS'
