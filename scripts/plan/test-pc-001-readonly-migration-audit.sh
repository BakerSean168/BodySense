#!/usr/bin/env bash
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
readonly_sql="$repo_root/scripts/plan/sql/pc-001-singleton-readiness.sql"
base_schema="$repo_root/apps/api/migrations/000001_vnext_baseline.up.sql"
image="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
container=""
cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# No host ports, host volumes or real user data. Remove only our own container.
container="$(docker run --rm -d \
  --name "bs-pc001-readonly-$$-$RANDOM" --network none \
  --memory 1g --cpus 1 --pids-limit 128 \
  --env POSTGRES_PASSWORD=synthetic-audit-only "$image")"
ready=0
# The official image briefly accepts connections during initdb and then
# stops that temporary server before starting the final postmaster. Merely
# observing pg_isready once can race this transition. Wait for the explicit
# init-completed marker and verify a real SQL query on the final instance.
for _ in $(seq 1 45); do
  if docker logs "$container" 2>&1 | grep -q 'PostgreSQL init process complete' &&
     docker exec "$container" psql -X -q -v ON_ERROR_STOP=1        -U postgres -d postgres -tAc 'SELECT 1' 2>/dev/null | grep -qx '1'; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != "1" ]]; then
  docker logs --tail 30 "$container" >&2
  echo "PG18 fixture not ready" >&2
  exit 1
fi
docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
  -U postgres -d postgres < "$base_schema" >/dev/null
fixture_sql() {
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres >/dev/null
}
audit_summary() {
  docker exec -i "$container" psql -X -t -A -F '|' \
    -v ON_ERROR_STOP=1 -U postgres -d postgres < "$readonly_sql" |
    awk -F '|' 'NF == 11 {print}'
}
expect_summary() {
  local label="$1" expected="$2" actual
  actual="$(audit_summary)"
  if [[ "$actual" != "$expected" ]]; then
    printf 'PC-001 %s FAIL: got <%s>, wanted <%s>\n' "$label" "$actual" "$expected" >&2
    exit 1
  fi
  printf 'PC-001 %s PASS: %s\n' "$label" "$actual"
}
# A user may have no Conversation until the first Chat access.
expect_summary empty '0|0|0|0|0|0|0|0|0|0|t'

fixture_sql <<'SQL'
INSERT INTO public.users(id,email,password_hash) VALUES
('00000000-0000-4000-8000-000000001001','pc1@example.invalid','synthetic'),
('00000000-0000-4000-8000-000000001002','pc2@example.invalid','synthetic'),
('00000000-0000-4000-8000-000000001003','pc3@example.invalid','synthetic');
INSERT INTO public.conversations(id,user_id,status,deleted_at) VALUES
('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001','active',NULL),
('00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000001001','active',NULL),
('00000000-0000-4000-8000-000000002003','00000000-0000-4000-8000-000000001001','active',now()),
('00000000-0000-4000-8000-000000002004','00000000-0000-4000-8000-000000001002','active',NULL);
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model) VALUES
('00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001','pc-run-1','00000000-0000-4000-8000-000000001001','waiting_user','test'),
('00000000-0000-4000-8000-000000003002','00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000004002','pc-run-2','00000000-0000-4000-8000-000000001001','running','test'),
('00000000-0000-4000-8000-000000003003','00000000-0000-4000-8000-000000002004','00000000-0000-4000-8000-000000004003','pc-run-3','00000000-0000-4000-8000-000000001003','completed','test');
UPDATE public.conversations
  SET active_run_id = '00000000-0000-4000-8000-000000003003'
  WHERE id = '00000000-0000-4000-8000-000000002001';
UPDATE public.conversations
  SET active_run_id = '00000000-0000-4000-8000-000000003002'
  WHERE id = '00000000-0000-4000-8000-000000002002';
INSERT INTO public.agent_interactions(id,run_id,conversation_id,tool_call_id,status)
VALUES (
'00000000-0000-4000-8000-000000005001',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000002001','pc-ask-user','pending');
INSERT INTO public.conversation_shares(id,conversation_id,share_token,snapshot_messages)
VALUES (
'00000000-0000-4000-8000-000000006001',
'00000000-0000-4000-8000-000000002002',
'synthetic-only-share-token-00001','[]');
SQL

# Duplicate conversations, parallel Runs, pending Ask User, foreign run owner,
# foreign active pointer and legacy public snapshot must all be detected.
expect_summary conflicting '3|1|3|1|1|2|1|1|1|1|f'

fixture_sql <<'SQL'
TRUNCATE public.conversations CASCADE;
DELETE FROM public.users;
INSERT INTO public.users(id,email,password_hash)
VALUES ('00000000-0000-4000-8000-000000001001','pc1@example.invalid','synthetic');
INSERT INTO public.conversations(id,user_id,status)
VALUES ('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001','active');
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model)
VALUES (
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001',
'pc-run-clean','00000000-0000-4000-8000-000000001001','completed','test');
SQL
expect_summary resolved '1|0|1|0|0|0|0|0|0|0|t'

# Soft-deleted history can still have source/run references and privacy obligations.
# Do not falsely mark it clear only because one currently live Conversation remains.
fixture_sql <<'SQL'
INSERT INTO public.conversations(id,user_id,status,deleted_at)
VALUES (
'00000000-0000-4000-8000-000000002003',
'00000000-0000-4000-8000-000000001001','active',now());
SQL
expect_summary deleted_history '1|0|1|1|0|0|0|0|0|0|f'
echo 'PC-001 native PostgreSQL 18 read-only migration audit: 4/4 PASS'
