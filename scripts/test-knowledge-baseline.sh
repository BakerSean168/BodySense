#!/usr/bin/env bash
# #510: disposable PG18 test for the aggregate-only knowledge baseline SQL.
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
name="bodysense-kb-baseline-$RANDOM-$$"
password="kb-baseline-synthetic"
report="$(mktemp)"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  rm -f "$report"
}
trap cleanup EXIT

docker run --rm -d \
  --name "$name" \
  -e POSTGRES_USER=bodysense \
  -e POSTGRES_PASSWORD="$password" \
  -e POSTGRES_DB=bodysense \
  -p 127.0.0.1::5432 \
  pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a >/dev/null

for _ in $(seq 1 50); do
  docker exec "$name" pg_isready -U bodysense -d bodysense >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$name" pg_isready -U bodysense -d bodysense >/dev/null
port="$(docker port "$name" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/' | head -1)"
if [[ ! "$port" =~ ^[0-9]+$ ]]; then
  echo "unable to resolve disposable PostgreSQL port" >&2
  exit 1
fi
(
  cd "$root/apps/api"
  go run ./cmd/migration-validator \
    -database-url "postgres://bodysense:$password@127.0.0.1:$port/bodysense?sslmode=disable" \
    -migrations file://migrations
)

# Inject a synthetic source only; note SQL above has no DML permissions.
docker exec -i "$name" psql -X -qAt -v ON_ERROR_STOP=1 -U bodysense -d bodysense <<'SQL'
INSERT INTO knowledge_sources
  (source_key, source_type, title, author, problem_slug, problem_display_name,
   original_file_path, ingest_status, license_status)
VALUES
  ('kb-baseline-test-only','video','synthetic fixture','fixture','knee','knee',
   'fixtures/none.mp4','pending','citation_only');
SQL

docker exec -i "$name" psql -X -qAt -v ON_ERROR_STOP=1 -v environment=synthetic \
  -U bodysense -d bodysense < "$root/scripts/knowledge-baseline.sql" > "$report"

python3 - "$report" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as fd:
    data = json.load(fd)
assert data["report_schema"] == "bodysense.knowledge-baseline.v1", data
assert data["declared_environment"] == "synthetic"
assert data["transaction_read_only"] is True
assert data["schema_migration"] >= 2
assert data["G1_source_types_and_licenses"] == [
    {"source_type": "video", "license_status": "citation_only", "ingest_status": "pending", "sources": 1}
], data["G1_source_types_and_licenses"]
assert data["G2_unit_lifecycle"] == [], data
assert data["G3_publications"]["total_registered_sources"] == 1
assert data["G3_publications"]["published_units"] == 0
assert data["G4_source_type_distribution"] == [{"source_type": "video", "sources": 1}]
assert data["G5_observations"] == []
assert data["G6_live_retrieval_hit_rate"] is None
assert data["G7_runtime_embedding_provider"] is None
print("KNOWLEDGE_BASELINE_READONLY_TEST=PASS")
PY

count="$(docker exec "$name" psql -X -qAt -U bodysense -d bodysense -c "SELECT count(*) FROM knowledge_sources;")"
[[ "$count" == "1" ]] || { echo "report unexpectedly modified sources" >&2; exit 1; }
