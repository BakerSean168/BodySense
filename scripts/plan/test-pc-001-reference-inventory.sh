#!/usr/bin/env bash
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
readonly_sql="$repo_root/scripts/plan/sql/pc-001-reference-inventory.sql"
base_schema="$repo_root/apps/api/migrations/000001_vnext_baseline.up.sql"
image="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
container=""
cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Cached digest only; no pulls, ports, mounts, production URLs or credentials.
docker image inspect "$image" >/dev/null
container="$(docker run --pull never --rm -d \
  --name "bs-pc001-inventory-$$-$RANDOM" --network none \
  --memory 1g --cpus 1 --pids-limit 128 \
  --env POSTGRES_PASSWORD=synthetic-inventory-only "$image")"
ready=0
for _ in $(seq 1 45); do
  if docker logs "$container" 2>&1 | rg -q 'PostgreSQL init process complete' &&
     docker exec "$container" psql -X -q -v ON_ERROR_STOP=1 \
       -U postgres -d postgres -tAc 'SELECT 1' 2>/dev/null | rg -qx '1'; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then
  echo 'PG18 synthetic fixture not ready' >&2
  exit 1
fi
fixture_sql() {
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres >/dev/null
}
fixture_sql < "$base_schema"
inventory() {
  docker exec -i "$container" psql -X -q -t -A -F '|' \
    -v ON_ERROR_STOP=1 -U postgres -d postgres < "$readonly_sql"
}
assert_reference() {
  local schema="$1" table="$2" column="$3" expected="$4" actual
  actual="$(printf '%s\n' "$report" | awk -F '|' \
    -v s="$schema" -v t="$table" -v c="$column" \
    'NF == 20 && $1 == s && $2 == t && $4 == c {print $7 "|" $16 "|" $17 "|" $18 "|" $19 "|" $20}')"
  if [[ "$actual" != "$expected" ]]; then
    printf 'FAIL %s.%s.%s: aggregate <%s>, expected <%s>\n' \
      "$schema" "$table" "$column" "$actual" "$expected" >&2
    exit 1
  fi
}
assert_private_output() {
  if printf '%s\n' "$report" | rg -qi \
    '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|inventory@example.invalid|SYNTHETIC_PRIVATE_SENTINEL'; then
    echo 'FAIL: inventory projected fixture data' >&2
    exit 1
  fi
}
baseline_tables=(agent_interactions agent_tool_calls ai_output_reviews
  consultation_rollout_observations consultation_sessions conversation_shares
  jobs messages runs runtime_events thread_projection_messages
  thread_projection_tool_calls thread_projections)
report="$(inventory)"
actual_tables="$(printf '%s\n' "$report" | awk -F '|' 'NF == 20 {print $2}')"
expected_tables="$(printf '%s\n' "${baseline_tables[@]}" | sort)"
[[ "$actual_tables" == "$expected_tables" ]] || { echo 'FAIL: baseline catalog coverage' >&2; exit 1; }
for table in "${baseline_tables[@]}"; do
  assert_reference public "$table" conversation_id 'foreign_key|0|0|0|0|0'
done
echo 'PC-001 inventory empty baseline PASS (13 FK tables)'

fixture_sql <<'SQL'
INSERT INTO public.users(id,email,password_hash)
VALUES ('00000000-0000-4000-8000-000000001001','inventory@example.invalid','synthetic');
INSERT INTO public.conversations(id,user_id,deleted_at) VALUES
('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001',NULL),
('00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000001001',now());
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model)
VALUES ('00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001','synthetic-run','00000000-0000-4000-8000-000000001001','waiting_user','test');
INSERT INTO public.messages(id,conversation_id,turn_id,role,seq,content_text)
VALUES ('00000000-0000-4000-8000-000000005001','00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001','user',1,'SYNTHETIC_PRIVATE_SENTINEL');
INSERT INTO public.agent_interactions(run_id,conversation_id,tool_call_id,question)
VALUES ('00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001','ask-test','{"sentinel":"SYNTHETIC_PRIVATE_SENTINEL"}');
INSERT INTO public.agent_tool_calls(run_id,conversation_id,tool_call_id,tool_name)
VALUES ('00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001','tool-test','ask_user');
INSERT INTO public.ai_output_reviews(conversation_id,output_type,raw_output) VALUES
(NULL,'synthetic','{"sentinel":"SYNTHETIC_PRIVATE_SENTINEL"}'),
('00000000-0000-4000-8000-000000002002','synthetic','{}');
INSERT INTO public.jobs(conversation_id,user_id,job_type) VALUES
(NULL,'00000000-0000-4000-8000-000000001001','synthetic'),
('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001','synthetic');
INSERT INTO public.consultation_rollout_observations(run_id,conversation_id,stage,champion_configuration_id,challenger_configuration_id)
VALUES ('00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001','synthetic','test','test');
INSERT INTO public.consultation_sessions(conversation_id)
VALUES ('00000000-0000-4000-8000-000000002001');
INSERT INTO public.conversation_shares(conversation_id,share_token,snapshot_messages)
VALUES ('00000000-0000-4000-8000-000000002002','SYNTHETIC_PRIVATE_SENTINEL','[{"text":"SYNTHETIC_PRIVATE_SENTINEL"}]');
INSERT INTO public.runtime_events(conversation_id,run_id,seq,channel,type)
VALUES ('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000003001',1,'synthetic','test');
INSERT INTO public.thread_projection_messages(message_id,conversation_id,turn_id,seq,role,status,created_at,updated_at)
VALUES ('00000000-0000-4000-8000-000000005001','00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001',1,'user','completed',now(),now());
INSERT INTO public.thread_projection_tool_calls(tool_call_id,conversation_id,run_id,tool_name,status,created_at,started_at)
VALUES ('projection-test','00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000003001','ask_user','running',now(),now());
INSERT INTO public.thread_projections(conversation_id,user_id,conversation_created_at,conversation_updated_at,session_created_at,session_updated_at)
VALUES ('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001',now(),now(),now(),now());
SQL
report="$(inventory)"
for table in "${baseline_tables[@]}"; do
  case "$table" in
    ai_output_reviews) expected='foreign_key|2|1|1|0|1' ;;
    jobs) expected='foreign_key|2|1|1|0|0' ;;
    conversation_shares) expected='foreign_key|1|0|1|0|1' ;;
    *) expected='foreign_key|1|0|1|0|0' ;;
  esac
  assert_reference public "$table" conversation_id "$expected"
done
assert_private_output
echo 'PC-001 inventory populated/null/deleted targets PASS (13 FK tables)'

# Synthetic drift only: non-FK source, text/non-UUID values, quoted identifiers,
# renamed composite FK and a NOT VALID FK with a pre-existing orphan.
fixture_sql <<'SQL'
CREATE SCHEMA inventory_fixture;
CREATE TABLE inventory_fixture.statement_sources(source_conversation_id uuid);
INSERT INTO inventory_fixture.statement_sources VALUES (NULL),
('00000000-0000-4000-8000-000000002001'),
('00000000-0000-4000-8000-000000009999');
CREATE TABLE inventory_fixture."ghost' table" (conversation_id text);
INSERT INTO inventory_fixture."ghost' table" VALUES (NULL),
('00000000-0000-4000-8000-000000002002'),('not-a-uuid');
ALTER TABLE public.conversations ADD UNIQUE (id, user_id);
CREATE TABLE inventory_fixture.renamed_reference (
  "local id" uuid, owner_id uuid,
  CONSTRAINT composite_reference FOREIGN KEY ("local id", owner_id)
    REFERENCES public.conversations(id,user_id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED
);
INSERT INTO inventory_fixture.renamed_reference VALUES
('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001');
CREATE TABLE inventory_fixture.unvalidated_reference (conversation_id uuid);
INSERT INTO inventory_fixture.unvalidated_reference VALUES ('00000000-0000-4000-8000-000000009999');
ALTER TABLE inventory_fixture.unvalidated_reference ADD CONSTRAINT pending_fk
  FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) NOT VALID;
SQL
report="$(inventory)"
assert_reference inventory_fixture statement_sources source_conversation_id 'no_conversation_fk|3|1|2|1|0'
assert_reference inventory_fixture "ghost' table" conversation_id 'no_conversation_fk|3|1|2|1|1'
assert_reference inventory_fixture renamed_reference 'local id' 'foreign_key|1|0|1|0|0'
assert_reference inventory_fixture unvalidated_reference conversation_id 'foreign_key|1|0|1|1|0'
printf '%s\n' "$report" | rg -Fq '|composite_reference|t|t|t|2|RESTRICT|NO ACTION|uuid|'
printf '%s\n' "$report" | rg -Fq '|pending_fk|f|f|f|1|NO ACTION|NO ACTION|uuid|'
printf '%s\n' "$report" | rg -Fq '|text_review_only|'
assert_private_output
echo 'PC-001 inventory non-FK/orphan/renamed/composite/unvalidated PASS'

# Assert the snapshot settings on the actual PG18 session and reject a write
# inserted before COMMIT. The failed statement contains no personal values.
settings="$(sed '/^COMMIT;/i SELECT current_setting('\''transaction_isolation'\''), current_setting('\''transaction_read_only'\''), current_setting('\''statement_timeout'\'');' "$readonly_sql" |
  docker exec -i "$container" psql -X -q -t -A -F '|' -U postgres -d postgres)"
printf '%s\n' "$settings" | rg -qx 'repeatable read\|on\|30s'
if failure="$(sed '/^COMMIT;/i INSERT INTO inventory_fixture.statement_sources VALUES (NULL);' "$readonly_sql" |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: inventory allowed mutation' >&2
  exit 1
fi
printf '%s\n' "$failure" | rg -Fq 'cannot execute INSERT in a read-only transaction'
report="$(inventory)"
assert_reference inventory_fixture statement_sources source_conversation_id 'no_conversation_fk|3|1|2|1|0'
echo 'PC-001 inventory snapshot/read-only enforcement PASS'

# An ordinary reader must not quietly see a partial result under RLS.
fixture_sql <<'SQL'
CREATE ROLE inventory_reader;
GRANT USAGE ON SCHEMA public, inventory_fixture TO inventory_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public, inventory_fixture TO inventory_reader;
ALTER TABLE inventory_fixture.statement_sources ENABLE ROW LEVEL SECURITY;
CREATE POLICY hidden_sources ON inventory_fixture.statement_sources USING (false);
SQL
if failure="$({ printf 'SET ROLE inventory_reader;\n'; cat "$readonly_sql"; } |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: inventory silently accepted RLS filtering' >&2
  exit 1
fi
printf '%s\n' "$failure" | rg -Fq 'query would be affected by row-level security policy'
fixture_sql <<'SQL'
ALTER TABLE inventory_fixture.statement_sources DISABLE ROW LEVEL SECURITY;
REVOKE SELECT ON inventory_fixture.statement_sources FROM inventory_reader;
SQL
if failure="$({ printf 'SET ROLE inventory_reader;\n'; cat "$readonly_sql"; } |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: inventory silently accepted missing permission' >&2
  exit 1
fi
printf '%s\n' "$failure" | rg -Fq 'permission denied for table statement_sources'
echo 'PC-001 inventory RLS incomplete-result rejection PASS'
echo 'PC-001 native PostgreSQL 18 reference inventory: 5/5 PASS'
