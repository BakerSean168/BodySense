#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
IMAGE="${PRIVACY_TEST_POSTGRES_IMAGE:-pgvector/pgvector:pg18}"
CONTAINER="bodysense-privacy-erasure-$$"
DB="bodysense_privacy_test"
PASSWORD="privacy-test"

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --name "$CONTAINER" \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB="$DB" \
  "$IMAGE" >/dev/null

for _ in $(seq 1 40); do
  if docker exec "$CONTAINER" pg_isready -U postgres -d "$DB" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$CONTAINER" pg_isready -U postgres -d "$DB" >/dev/null

for migration in $(find "$ROOT/apps/api/migrations" -maxdepth 1 -type f -name '*.up.sql' | sort -V); do
  docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null < "$migration"
done

# The newest migration must be reversible/replayable before testing its runtime semantics.
docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null \
  < "$ROOT/apps/api/migrations/000054_add_privacy_erasure_boundary.down.sql"
docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null \
  < "$ROOT/apps/api/migrations/000054_add_privacy_erasure_boundary.up.sql"

IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CONTAINER")"
(
  cd "$ROOT/apps/api"
  BODYSENSE_INTEGRATION_DATABASE_URL="host=$IP user=postgres password=$PASSWORD dbname=$DB port=5432 sslmode=disable" \
    go test ./internal/service -run '^TestPrivacyErasureSyntheticUserPostgres$' -count=1 -v
)

echo 'PRIVACY_ERASURE_INTEGRATION=PASS'
