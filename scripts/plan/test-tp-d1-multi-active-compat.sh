#!/usr/bin/env bash
# TP-D1 synthetic PostgreSQL 18 admission probe; no network ports or host mounts.
# legacy mode proves the current physical incompatibility; v2 mode is the
# future acceptance test and MUST fail while the legacy unique index survives.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
mode="${TP_D1_EXPECTATION:-legacy}"
if [[ -z "$mode" ]]; then
  mode="legacy"
fi
case "$mode" in
  legacy|v2) ;;
  *) echo 'TP-D1: expectation must be legacy or v2' >&2; exit 2 ;;
esac

# Refuse silently substituting a different local image or downloading one.
docker image inspect "$image" >/dev/null
container="bs-tpd1-compat-$$-$RANDOM"
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

# pg_isready during initdb can succeed before the final postmaster starts.
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
  docker logs --tail 35 "$container" >&2
  echo 'TP-D1: final PG18 instance was not ready' >&2
  exit 1
fi

migration_count=0
while IFS= read -r sql_file; do
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -U postgres -d synthetic_tpd1 < "$sql_file" >/dev/null
  migration_count=$((migration_count + 1))
done < <(find "$repo_root/apps/api/migrations" -maxdepth 1 -type f \
  -name '*.up.sql' | sort -V)
if [[ "$migration_count" -lt 1 ]]; then
  echo 'TP-D1: empty canonical migration set' >&2
  exit 1
fi
printf 'TP_D1_CANONICAL_MIGRATIONS=%s\n' "$migration_count"

sql() {
  docker exec -i "$container" psql -X -q -t -A -v ON_ERROR_STOP=1 \
    -U postgres -d synthetic_tpd1
}
# The independently keyed TreatmentRevision index is never removed.
revision_index="$(sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM pg_indexes
WHERE schemaname='public' AND tablename='training_plans'
  AND indexname='idx_training_plans_treatment_revision';
SQL
)"
if [[ "$revision_index" != "1" ]]; then
  echo 'TP-D1: treatment revision idempotency index missing' >&2
  exit 1
fi

legacy_index="$(sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM pg_indexes
WHERE schemaname='public' AND tablename='training_plans'
  AND indexname='idx_training_plans_user_active';
SQL
)"
if [[ "$mode" == "legacy" && "$legacy_index" != "1" ]]; then
  echo 'TP-D1: expected legacy single-active index is missing' >&2
  exit 1
fi
if [[ "$mode" == "v2" && "$legacy_index" != "0" ]]; then
  echo 'TP-D1 V2 BLOCKED: legacy single-active index still exists' >&2
  exit 1
fi

# Both modes use synthetic rows in rolled-back transactions. No persistent
# migration, record or privileged external connection can be affected.
if [[ "$mode" == "legacy" ]]; then
  sql <<'SQL'
BEGIN;
INSERT INTO public.users(id,email,password_hash) VALUES
('10000000-0000-4000-8000-000000001001','tpd1-a@example.invalid','synthetic'),
('10000000-0000-4000-8000-000000001002','tpd1-b@example.invalid','synthetic');
INSERT INTO public.training_plans(user_id,goal,status) VALUES
('10000000-0000-4000-8000-000000001001','synthetic A','active'),
('10000000-0000-4000-8000-000000001002','independent owner','active');
DO $guard$
DECLARE violated_name text;
BEGIN
  BEGIN
    INSERT INTO public.training_plans(user_id,goal,status) VALUES
    ('10000000-0000-4000-8000-000000001001','synthetic B','active');
    RAISE EXCEPTION 'TP-D1: legacy index unexpectedly allowed two active plans';
  EXCEPTION WHEN unique_violation THEN
    GET STACKED DIAGNOSTICS violated_name = CONSTRAINT_NAME;
    IF violated_name <> 'idx_training_plans_user_active' THEN
      RAISE EXCEPTION 'TP-D1: unexpected constraint %', violated_name;
    END IF;
    RAISE NOTICE 'TP_D1_LEGACY_REJECTION_CONFIRMED=%', violated_name;
  END;
END $guard$;
DO $guard$
DECLARE owned bigint; other_owner bigint;
BEGIN
  SELECT count(*) INTO owned FROM public.training_plans
    WHERE user_id='10000000-0000-4000-8000-000000001001' AND status='active';
  SELECT count(*) INTO other_owner FROM public.training_plans
    WHERE user_id='10000000-0000-4000-8000-000000001002' AND status='active';
  IF owned<>1 OR other_owner<>1 THEN
    RAISE EXCEPTION 'TP-D1: legacy active counts wrong % %', owned, other_owner;
  END IF;
END $guard$;
ROLLBACK;
SQL
  # Demonstrate why migration has to remove the index, without changing even
  # this fixture's persistent schema: the DROP itself is rolled back.
  sql <<'SQL'
BEGIN;
INSERT INTO public.users(id,email,password_hash) VALUES
('10000000-0000-4000-8000-000000001001','tpd1-a@example.invalid','synthetic');
INSERT INTO public.training_plans(user_id,goal,status) VALUES
('10000000-0000-4000-8000-000000001001','synthetic A','active');
DROP INDEX public.idx_training_plans_user_active;
INSERT INTO public.training_plans(user_id,goal,status) VALUES
('10000000-0000-4000-8000-000000001001','synthetic B','active');
DO $guard$
DECLARE count_active bigint;
BEGIN
  SELECT count(*) INTO count_active FROM public.training_plans
    WHERE user_id='10000000-0000-4000-8000-000000001001' AND status='active';
  IF count_active <> 2 THEN
    RAISE EXCEPTION 'TP-D1: expected two active plans after isolated index drop';
  END IF;
  RAISE NOTICE 'TP_D1_ISOLATED_COMPATIBILITY_COUNT=%', count_active;
END $guard$;
ROLLBACK;
SQL
  restored="$(sql <<'SQL' | tr -d '[:space:]'
SELECT count(*) FROM pg_indexes WHERE schemaname='public'
  AND tablename='training_plans'
  AND indexname='idx_training_plans_user_active';
SQL
)"
  test "$restored" = 1
  echo 'TP_D1_LEGACY_BLOCKER_REPRODUCED=PASS'
else
  sql <<'SQL'
BEGIN;
INSERT INTO public.users(id,email,password_hash) VALUES
('10000000-0000-4000-8000-000000001001','tpd1-a@example.invalid','synthetic'),
('10000000-0000-4000-8000-000000001002','tpd1-b@example.invalid','synthetic');
INSERT INTO public.training_plans(user_id,goal,status) VALUES
('10000000-0000-4000-8000-000000001001','synthetic A','active'),
('10000000-0000-4000-8000-000000001001','synthetic B','active'),
('10000000-0000-4000-8000-000000001002','independent owner','active');
DO $guard$
DECLARE own_active bigint; foreign_active bigint;
BEGIN
  SELECT count(*) INTO own_active FROM public.training_plans
    WHERE user_id='10000000-0000-4000-8000-000000001001' AND status='active';
  SELECT count(*) INTO foreign_active FROM public.training_plans
    WHERE user_id='10000000-0000-4000-8000-000000001002' AND status='active';
  IF own_active <> 2 OR foreign_active <> 1 THEN
    RAISE EXCEPTION 'TP-D1 V2: wrong active plan counts % %', own_active, foreign_active;
  END IF;
END $guard$;
ROLLBACK;
SQL
  echo 'TP_D1_MULTI_ACTIVE_DB_COMPATIBILITY=PASS'
fi
echo 'TP_D1_SYNTHETIC_PG18_PREFLIGHT_PASS'
