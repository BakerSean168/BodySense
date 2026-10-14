#!/usr/bin/env bash
# Synthetic-only TP-D1 forward/rollback rehearsal. Never connects to a live DB.
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
up="$repo_root/scripts/plan/sql/tp-d1-multi-active-forward-preview.sql"
down="$repo_root/scripts/plan/sql/tp-d1-multi-active-reverse-preview.sql"
docker image inspect "$image" >/dev/null
container="bs-tpd1-preview-$$-$RANDOM"
cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
docker run --pull never --rm -d --name "$container" --network none \
  --memory 1g --cpus 1 --pids-limit 128 \
  -e POSTGRES_PASSWORD=synthetic-tpd1-only \
  -e POSTGRES_DB=synthetic_tpd1 "$image" >/dev/null
ready=0
for _ in $(seq 1 45); do
  if docker logs "$container" 2>&1 | grep -q 'PostgreSQL init process complete' &&
     docker exec "$container" psql -X -q -v ON_ERROR_STOP=1 \
       -U postgres -d synthetic_tpd1 -tAc 'SELECT 1' 2>/dev/null | grep -qx 1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != "1" ]]; then
  echo 'TP-D1 preview PG18 final postmaster not ready' >&2
  exit 1
fi
migrations=0
while IFS= read -r sql_file; do
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d synthetic_tpd1 < "$sql_file" >/dev/null
  migrations=$((migrations + 1))
done < <(find "$repo_root/apps/api/migrations" -maxdepth 1 -type f \
  -name '*.up.sql' | sort -V)
if [[ "$migrations" -lt 12 ]]; then
  echo "TP-D1 preview expected at least 12 canonical migrations, got $migrations" >&2
  exit 1
fi
printf 'TP_D1_MIGRATIONS=%s\n' "$migrations"
sql() {
  docker exec -i "$container" psql -X -q -t -A -v ON_ERROR_STOP=1 \
    -U postgres -d synthetic_tpd1
}
index_exists() {
  sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM pg_indexes WHERE schemaname='public'
  AND tablename='training_plans' AND indexname='idx_training_plans_user_active';
SQL
}
rev_index_exists() {
  sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM pg_indexes WHERE schemaname='public'
  AND tablename='training_plans' AND indexname='idx_training_plans_treatment_revision';
SQL
}
test "$(index_exists)" = 1
test "$(rev_index_exists)" = 1
sql < "$up" >/dev/null
test "$(index_exists)" = 0
test "$(rev_index_exists)" = 1
sql <<'SQL' >/dev/null
INSERT INTO users(id,email,password_hash) VALUES
('10000000-0000-4000-8000-000000000001','tpd1-preview-a@example.invalid','synthetic'),
('10000000-0000-4000-8000-000000000002','tpd1-preview-b@example.invalid','synthetic');
INSERT INTO training_plans(id,user_id,status,goal) VALUES
('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','active','synthetic A'),
('20000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','active','synthetic B'),
('20000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000002','active','synthetic independent owner');
SQL
check_three_plans() {
  sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM training_plans WHERE status='active';
SQL
}
test "$(check_three_plans)" = 3
echo 'TP_D1_FORWARD_TWO_SAME_OWNER_ACTIVE=PASS'

# The preview down migration MUST refuse to erase or overwrite legitimate
# accepted V2 plan status. PostgreSQL single-transaction mode prohibits
# partial mutation even if an index statement later fails.
set +e
reverse_output="$(docker exec -i "$container" psql -X -q -1 -v ON_ERROR_STOP=1 \
  -U postgres -d synthetic_tpd1 < "$down" 2>&1)"
reverse_status=$?
set -e
if [[ "$reverse_status" -eq 0 ]] ||
   [[ "$reverse_output" != *'TP-D1 reverse blocked: multiple active plans'* ]]; then
  echo "TP-D1 reverse incorrectly accepted multi-active history: $reverse_output" >&2
  exit 1
fi
test "$(index_exists)" = 0
test "$(rev_index_exists)" = 1
test "$(check_three_plans)" = 3
echo 'TP_D1_REVERSE_FAIL_CLOSED_WITH_MULTIPLE_ACTIVE=PASS'

# Synthetic-only operator-reconciliation rehearsal: explicitly pause the
# second synthetic plan. This is NOT permission to change real users' plans.
sql <<'SQL' >/dev/null
UPDATE training_plans SET status='paused'
WHERE id='20000000-0000-4000-8000-000000000002'
  AND user_id='10000000-0000-4000-8000-000000000001';
SQL
docker exec -i "$container" psql -X -q -1 -v ON_ERROR_STOP=1 \
  -U postgres -d synthetic_tpd1 < "$down" >/dev/null
test "$(index_exists)" = 1
test "$(rev_index_exists)" = 1
test "$(check_three_plans)" = 2
echo 'TP_D1_SYNTHETIC_RECONCILED_REVERSE=PASS'
echo 'TP_D1_OFFLINE_MIGRATION_PREVIEW=PASS'
