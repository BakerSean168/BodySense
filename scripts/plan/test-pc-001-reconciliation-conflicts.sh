#!/usr/bin/env bash
# Synthetic PG18 fixture only: never connect to a user database.
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
readonly_sql="$repo_root/scripts/plan/sql/pc-001-reconciliation-conflicts.sql"
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

# Pinned cached image; no network, mounts, ports, volumes or real credentials.
docker image inspect "$image" >/dev/null
container="$(docker run --pull never --rm -d \
  --name "bs-pc001-conflicts-$$-$RANDOM" --network none \
  --memory 1g --cpus 1 --pids-limit 128 \
  --env POSTGRES_PASSWORD=synthetic-conflicts-only "$image")"
ready=0
for _ in $(seq 1 45); do
  if docker logs "$container" 2>&1 | grep -q 'PostgreSQL init process complete' &&
    docker exec "$container" psql -X -q -v ON_ERROR_STOP=1 \
      -U postgres -d postgres -tAc 'SELECT 1' 2>/dev/null | grep -qx 1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then
  echo 'PC-001 conflicts synthetic PG18 fixture not ready' >&2
  exit 1
fi
fixture_sql() {
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres >/dev/null
}
audit() {
  docker exec -i "$container" psql -X -q -t -A -F '|' \
    -v ON_ERROR_STOP=1 -U postgres -d postgres < "$readonly_sql"
}
assert_private() {
  if printf '%s\n' "$report" | grep -Eq \
    '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|SYNTHETIC_PRIVATE_SENTINEL|pc-reconcile@example.invalid'; then
    echo 'FAIL: raw identifier or private fixture data projected' >&2
    exit 1
  fi
}
expect_counts() {
  local label="$1" expected="$2" actual
  report="$(audit)"
  actual="$(printf '%s\n' "$report" | awk -F '|' 'NF == 11 {print}')"
  if [[ "$actual" != "$expected" ]]; then
    printf 'FAIL %s got <%s> expected <%s>\n' "$label" "$actual" "$expected" >&2
    exit 1
  fi
  assert_private
  printf 'PC-001 reconciliation %s PASS: %s\n' "$label" "$actual"
}

# Old baseline without interaction-resume/source migrations must fail closed.
fixture_sql < "$repo_root/apps/api/migrations/000001_vnext_baseline.up.sql"
if failure="$(audit 2>&1)"; then
  echo 'FAIL: old schema incorrectly accepted for the reconciliation audit' >&2
  exit 1
fi
printf '%s\n' "$failure" | grep -Eq \
  'relation .*does not exist|division by zero|statement_evidence'
echo 'PC-001 reconciliation missing migration fail-closed PASS'

# Rebuild only the synthetic fixture; apply every current canonical migration
# in numeric order to detect drift in post-000006 source and receipt contracts.
for f in "$repo_root"/apps/api/migrations/*.up.sql; do
  case "$(basename "$f")" in
    000001_*) continue ;;
  esac
  fixture_sql < "$f"
done
expect_counts empty_schema '0|0|0|0|0|0|0|0|0|0|0'

# One well-formed Conversation, original Run, source excerpt, Ask User receipt,
# projection and session: no false-positive migration blockers.
fixture_sql <<'SQL'
INSERT INTO public.users(id,email,password_hash) VALUES
('00000000-0000-4000-8000-000000001001','pc-reconcile@example.invalid','synthetic'),
('00000000-0000-4000-8000-000000001002','other-reconcile@example.invalid','synthetic');
INSERT INTO public.conversations(id,user_id,status) VALUES
('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001','active');
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model)
VALUES ('00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'synthetic-run-one','00000000-0000-4000-8000-000000001001','completed','test');
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq,content_text) VALUES
('00000000-0000-4000-8000-000000005001','00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001','00000000-0000-4000-8000-000000003001',
'user',1,'SYNTHETIC_PRIVATE_SENTINEL'),
('00000000-0000-4000-8000-000000005002','00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000004001','00000000-0000-4000-8000-000000003001',
'assistant',2,'synthetic');
INSERT INTO public.agent_interactions(id,run_id,conversation_id,tool_call_id,status,answer)
VALUES ('00000000-0000-4000-8000-000000006001',
'00000000-0000-4000-8000-000000003001','00000000-0000-4000-8000-000000002001',
'ask-one','answered','{"answer":"synthetic"}');
INSERT INTO public.interaction_resumes(interaction_id,user_id,conversation_id,run_id,
request_id,answer_fingerprint,question_fingerprint,user_message_id,assistant_message_id,claimed_at)
VALUES ('00000000-0000-4000-8000-000000006001','00000000-0000-4000-8000-000000001001',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000003001',
'synthetic-resume-one',repeat('a',64),repeat('b',64),
'00000000-0000-4000-8000-000000005001','00000000-0000-4000-8000-000000005002',now());
INSERT INTO public.statement_evidence
(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,
source_conversation_id,source_run_id,source_message_id)
VALUES ('00000000-0000-4000-8000-000000007001','00000000-0000-4000-8000-000000001001',
'user_message',1,'SYNTHETIC_PRIVATE_SENTINEL','available',now(),
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000005001');
INSERT INTO public.consultation_sessions(conversation_id)
VALUES ('00000000-0000-4000-8000-000000002001');
INSERT INTO public.thread_projections
(conversation_id,user_id,conversation_created_at,conversation_updated_at,
session_created_at,session_updated_at)
VALUES ('00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000001001',
now(),now(),now(),now());
INSERT INTO public.thread_projection_messages
(message_id,conversation_id,turn_id,run_id,seq,role,status,created_at,updated_at)
VALUES ('00000000-0000-4000-8000-000000005001',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'00000000-0000-4000-8000-000000003001',1,'user','completed',now(),now());
SQL
expect_counts consistent_single '0|0|0|0|0|0|0|0|0|0|0'

# Merging old histories without planning a stable message order or handling
# per-Conversation primary keys would fail even if all FK targets exist.
fixture_sql <<'SQL'
INSERT INTO public.conversations(id,user_id,status)
VALUES ('00000000-0000-4000-8000-000000002002',
'00000000-0000-4000-8000-000000001001','active');
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model)
VALUES ('00000000-0000-4000-8000-000000003002',
'00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000004002',
'synthetic-run-two','00000000-0000-4000-8000-000000001001','completed','test');
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq) VALUES
('00000000-0000-4000-8000-000000005003',
'00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000004002',
'00000000-0000-4000-8000-000000003002','user',1);
INSERT INTO public.consultation_sessions(conversation_id)
VALUES ('00000000-0000-4000-8000-000000002002');
INSERT INTO public.thread_projections
(conversation_id,user_id,conversation_created_at,conversation_updated_at,
session_created_at,session_updated_at)
VALUES ('00000000-0000-4000-8000-000000002002',
'00000000-0000-4000-8000-000000001001',now(),now(),now(),now());
INSERT INTO public.thread_projection_messages
(message_id,conversation_id,turn_id,run_id,seq,role,status,created_at,updated_at)
VALUES ('00000000-0000-4000-8000-000000005003',
'00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000004002',
'00000000-0000-4000-8000-000000003002',1,'user','completed',now(),now());
INSERT INTO public.conversation_shares(conversation_id,share_token,snapshot_messages)
VALUES ('00000000-0000-4000-8000-000000002002',
'synthetic-share-token-for-test','[{"text":"SYNTHETIC_PRIVATE_SENTINEL"}]');
SQL
expect_counts history_merge_conflicts '1|1|1|1|0|0|0|0|0|0|1'

# Long-lived evidence records use NON-FK source keys and can outlive pruned
# messages. Existing conflicting identities are unsafe; missing source ids
# require a separate historical/pruning reconciliation, not blind deletion.
fixture_sql <<'SQL'
INSERT INTO public.conversations(id,user_id,status)
VALUES ('00000000-0000-4000-8000-000000002003',
'00000000-0000-4000-8000-000000001002','active');
INSERT INTO public.runs(id,conversation_id,turn_id,request_id,user_id,status,model)
VALUES ('00000000-0000-4000-8000-000000003003',
'00000000-0000-4000-8000-000000002003','00000000-0000-4000-8000-000000004003',
'synthetic-run-three','00000000-0000-4000-8000-000000001002','completed','test');
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq)
VALUES ('00000000-0000-4000-8000-000000005004',
'00000000-0000-4000-8000-000000002003','00000000-0000-4000-8000-000000004003',
'00000000-0000-4000-8000-000000003003','user',1);
INSERT INTO public.agent_interactions(id,run_id,conversation_id,tool_call_id,status)
VALUES ('00000000-0000-4000-8000-000000006002',
'00000000-0000-4000-8000-000000003003',
'00000000-0000-4000-8000-000000002003','ask-unreceipted','answered');
INSERT INTO public.statement_evidence
(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,
source_conversation_id,source_run_id,source_message_id)
VALUES
('00000000-0000-4000-8000-000000007002','00000000-0000-4000-8000-000000001001',
'user_message',1,'SYNTHETIC_PRIVATE_SENTINEL','available',now(),
'00000000-0000-4000-8000-000000002003',
'00000000-0000-4000-8000-000000003003',
'00000000-0000-4000-8000-000000005004'),
('00000000-0000-4000-8000-000000007003','00000000-0000-4000-8000-000000001001',
'user_message',1,'synthetic historic', 'available',now(),
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000009999');
INSERT INTO public.statement_evidence
(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,
source_conversation_id,source_run_id,source_message_id,
source_interaction_id,source_tool_call_id,question_excerpt)
VALUES ('00000000-0000-4000-8000-000000007004',
'00000000-0000-4000-8000-000000001001',
'structured_answer',1,'synthetic structured','available',now(),
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000005001',
'00000000-0000-4000-8000-000000006002','ask-unreceipted','synthetic question');
SQL
expect_counts provenance_legacy '1|1|1|1|2|1|0|1|0|0|1'

# A syntactically valid FK receipt can point at the WRONG original Run.
# The database MUST reject reusing receipt message IDs, but unique message
# ownership alone does not guarantee correct interaction/run binding.
fixture_sql <<'SQL'
INSERT INTO public.agent_interactions(id,run_id,conversation_id,tool_call_id,status)
VALUES ('00000000-0000-4000-8000-000000006003',
'00000000-0000-4000-8000-000000003002',
'00000000-0000-4000-8000-000000002002','ask-stale','pending');
SQL
if failure="$(docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
  -U postgres -d postgres 2>&1 <<'SQL'
INSERT INTO public.interaction_resumes(interaction_id,user_id,conversation_id,run_id,
request_id,answer_fingerprint,question_fingerprint,user_message_id,assistant_message_id,claimed_at)
VALUES ('00000000-0000-4000-8000-000000006003',
'00000000-0000-4000-8000-000000001001',
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'synthetic-wrong-resume',repeat('c',64),repeat('d',64),
'00000000-0000-4000-8000-000000005001',
'00000000-0000-4000-8000-000000005002',now());
SQL
)"; then
  echo 'FAIL: duplicate answer message IDs accepted' >&2
  exit 1
fi
printf '%s\n' "$failure" | grep -Fq 'duplicate key value violates unique constraint'
fixture_sql <<'SQL'
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq) VALUES
('00000000-0000-4000-8000-000000005005',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'00000000-0000-4000-8000-000000003001','user',3),
('00000000-0000-4000-8000-000000005006',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'00000000-0000-4000-8000-000000003001','assistant',4);
INSERT INTO public.interaction_resumes(interaction_id,user_id,conversation_id,run_id,
request_id,answer_fingerprint,question_fingerprint,user_message_id,assistant_message_id,claimed_at)
VALUES ('00000000-0000-4000-8000-000000006003',
'00000000-0000-4000-8000-000000001001',
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'synthetic-wrong-resume',repeat('c',64),repeat('d',64),
'00000000-0000-4000-8000-000000005005',
'00000000-0000-4000-8000-000000005006',now());
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq)
VALUES ('00000000-0000-4000-8000-000000005007',
'00000000-0000-4000-8000-000000002002','00000000-0000-4000-8000-000000004002',
'00000000-0000-4000-8000-000000003002','user',2);
INSERT INTO public.thread_projection_messages
(message_id,conversation_id,turn_id,run_id,seq,role,status,created_at,updated_at)
VALUES ('00000000-0000-4000-8000-000000005007',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004002',
'00000000-0000-4000-8000-000000003001',2,'user','completed',now(),now());
SQL

# Source provenance can match the original owner/Run/Message/Interaction but
# forge a DIFFERENT toolCallId. This is distinct from cross-owner linkage.
fixture_sql <<'SQL'
INSERT INTO public.messages(id,conversation_id,turn_id,run_id,role,seq) VALUES
('00000000-0000-4000-8000-000000005008',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'00000000-0000-4000-8000-000000003001','user',5),
('00000000-0000-4000-8000-000000005009',
'00000000-0000-4000-8000-000000002001','00000000-0000-4000-8000-000000004001',
'00000000-0000-4000-8000-000000003001','assistant',6);
INSERT INTO public.agent_interactions(id,run_id,conversation_id,tool_call_id,status)
VALUES ('00000000-0000-4000-8000-000000006004',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000002001','ask-original','answered');
INSERT INTO public.interaction_resumes(interaction_id,user_id,conversation_id,run_id,
request_id,answer_fingerprint,question_fingerprint,user_message_id,assistant_message_id,claimed_at)
VALUES ('00000000-0000-4000-8000-000000006004',
'00000000-0000-4000-8000-000000001001',
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'synthetic-forged-tool-call',repeat('e',64),repeat('f',64),
'00000000-0000-4000-8000-000000005008',
'00000000-0000-4000-8000-000000005009',now());
INSERT INTO public.statement_evidence
(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,
source_conversation_id,source_run_id,source_message_id,
source_interaction_id,source_tool_call_id,question_excerpt)
VALUES ('00000000-0000-4000-8000-000000007005',
'00000000-0000-4000-8000-000000001001','structured_answer',
1,'SYNTHETIC_PRIVATE_SENTINEL','available',now(),
'00000000-0000-4000-8000-000000002001',
'00000000-0000-4000-8000-000000003001',
'00000000-0000-4000-8000-000000005008',
'00000000-0000-4000-8000-000000006004',
'ask-forged','synthetic question');
SQL
expect_counts wrong_run_and_projection '1|1|1|1|3|1|1|1|1|1|1'

# Validate the SAME session settings used by the actual aggregate query.
settings="$(sed '/^COMMIT;/i SELECT current_setting('\''transaction_isolation'\''), current_setting('\''transaction_read_only'\''), current_setting('\''statement_timeout'\'');' "$readonly_sql" |
  docker exec -i "$container" psql -X -q -t -A -F '|' \
    -v ON_ERROR_STOP=1 -U postgres -d postgres)"
printf '%s\n' "$settings" | grep -qx 'repeatable read|on|30s'
if failure="$(sed '/^COMMIT;/i INSERT INTO public.conversations(user_id) VALUES (NULL);' "$readonly_sql" |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: reconciliation audit allowed mutation' >&2
  exit 1
fi
printf '%s\n' "$failure" | grep -Fq 'cannot execute INSERT in a read-only transaction'
expect_counts immutable_snapshot '1|1|1|1|3|1|1|1|1|1|1'

# Neither RLS-filtered nor denied metadata scans may be accepted as complete.
fixture_sql <<'SQL'
CREATE ROLE pc_conflict_reader;
GRANT USAGE ON SCHEMA public TO pc_conflict_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO pc_conflict_reader;
ALTER TABLE public.statement_evidence ENABLE ROW LEVEL SECURITY;
CREATE POLICY hidden_evidence ON public.statement_evidence USING (false);
SQL
if failure="$({ printf 'SET ROLE pc_conflict_reader;\n'; cat "$readonly_sql"; } |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: RLS-filtered provenance accepted' >&2
  exit 1
fi
printf '%s\n' "$failure" | grep -Fq 'query would be affected by row-level security policy'
fixture_sql <<'SQL'
ALTER TABLE public.statement_evidence DISABLE ROW LEVEL SECURITY;
REVOKE SELECT ON public.statement_evidence FROM pc_conflict_reader;
SQL
if failure="$({ printf 'SET ROLE pc_conflict_reader;\n'; cat "$readonly_sql"; } |
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d postgres 2>&1 >/dev/null)"; then
  echo 'FAIL: incomplete permission-scoped provenance accepted' >&2
  exit 1
fi
printf '%s\n' "$failure" | grep -Fq 'permission denied for table statement_evidence'
echo 'PC-001 reconciliation RLS/permission fail-closed PASS'
echo 'PC-001 native PostgreSQL 18 reconciliation conflicts: 8/8 PASS'
