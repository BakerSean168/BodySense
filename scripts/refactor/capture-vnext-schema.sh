#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${BODYSENSE_SCHEMA_SNAPSHOT_OUT:-$ROOT/docs/refactor/vnext/schema/database-schema.json}"
mkdir -p "$(dirname "$OUT")"
NAME="bodysense-vnext-baseline-pg-$$"
PASSWORD="bodysense-baseline"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run --rm -d \
  --name "$NAME" \
  -e POSTGRES_USER=bodysense \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB=bodysense \
  -p 127.0.0.1::5432 \
  pgvector/pgvector:pg18 >/dev/null

for _ in $(seq 1 60); do
  if docker exec "$NAME" pg_isready -U bodysense -d bodysense >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$NAME" pg_isready -U bodysense -d bodysense >/dev/null

PORT="$(docker port "$NAME" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/' | head -1)"
if [[ ! "$PORT" =~ ^[0-9]+$ ]]; then
  echo "failed to resolve disposable PostgreSQL port" >&2
  exit 1
fi

for _ in $(seq 1 30); do
  if (echo >"/dev/tcp/127.0.0.1/$PORT") >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! (echo >"/dev/tcp/127.0.0.1/$PORT") >/dev/null 2>&1; then
  echo "disposable PostgreSQL host port did not become reachable" >&2
  exit 1
fi
sleep 1

(
  cd "$ROOT/apps/api"
  go run ./cmd/migration-validator \
    -database-url "postgres://bodysense:${PASSWORD}@127.0.0.1:${PORT}/bodysense?sslmode=disable" \
    -migrations file://migrations
)

docker exec "$NAME" psql -U bodysense -d bodysense -X -q -t -A -c "
SELECT jsonb_pretty(jsonb_build_object(
  'schema_version', 1,
  'runtime_image', 'pgvector/pgvector:pg18',
  'migration_state', (SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1),
  'tables', (
    SELECT jsonb_agg(jsonb_build_object(
      'table', t.table_name,
      'columns', (
        SELECT jsonb_agg(jsonb_build_object(
          'name', c.column_name,
          'type', c.data_type,
          'udt', c.udt_name,
          'nullable', c.is_nullable = 'YES',
          'default', c.column_default
        ) ORDER BY c.ordinal_position)
        FROM information_schema.columns c
        WHERE c.table_schema='public' AND c.table_name=t.table_name
      )
    ) ORDER BY t.table_name)
    FROM information_schema.tables t
    WHERE t.table_schema='public' AND t.table_type='BASE TABLE'
  )
));" > "$OUT"

node -e "const d=require(process.argv[1]); console.log('schema snapshot:', d.migration_state, d.tables.length+' tables', d.tables.reduce((n,t)=>n+t.columns.length,0)+' columns')" "$OUT"
