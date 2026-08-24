#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
TMP_ROOT="$(mktemp -d)"
PROJECT="bodysense-preflight-$RANDOM-$$"
DB="bodysense_preflight"
PASSWORD="preflight-test"
IMAGE="${RUNTIME_TEST_POSTGRES_IMAGE:-pgvector/pgvector:pg18}"

cleanup() {
  BODYSENSE_DEPLOY_ROOT="$TMP_ROOT" BODYSENSE_COMPOSE_PROJECT="$PROJECT" \
    docker compose -p "$PROJECT" -f "$TMP_ROOT/docker/docker-compose.prod.yml" \
      --env-file "$TMP_ROOT/.env.production" --env-file "$TMP_ROOT/.env.production.local" \
      down -v >/dev/null 2>&1 || true
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

mkdir -p "$TMP_ROOT/docker" "$TMP_ROOT/scripts"
cp "$ROOT/scripts/production-deploy-watch.sh" "$TMP_ROOT/scripts/production-deploy-watch.sh"
cat > "$TMP_ROOT/.env.production" <<ENV
DB_USER=postgres
DB_NAME=$DB
ENV
cat > "$TMP_ROOT/.env.production.local" <<ENV
POSTGRES_PASSWORD=$PASSWORD
ENV
cat > "$TMP_ROOT/docker/docker-compose.prod.yml" <<YAML
services:
  postgres:
    image: $IMAGE
    environment:
      POSTGRES_PASSWORD: $PASSWORD
      POSTGRES_DB: $DB
YAML

compose() {
  docker compose -p "$PROJECT" -f "$TMP_ROOT/docker/docker-compose.prod.yml" \
    --env-file "$TMP_ROOT/.env.production" --env-file "$TMP_ROOT/.env.production.local" "$@"
}

compose up -d postgres >/dev/null
for _ in $(seq 1 40); do
  compose exec -T postgres pg_isready -U postgres -d "$DB" >/dev/null 2>&1 && break
  sleep 1
done
compose exec -T postgres pg_isready -U postgres -d "$DB" >/dev/null

for migration in $(find "$ROOT/apps/api/migrations" -maxdepth 1 -type f -name '*.up.sql' | sort -V); do
  compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null < "$migration"
done

USER_ID="00000000-0000-4000-8000-000000000111"
CONV_ID="00000000-0000-4000-8000-000000000222"
RUN_ID="00000000-0000-4000-8000-000000000333"
TURN_ID="00000000-0000-4000-8000-000000000444"
compose exec -T postgres psql -v ON_ERROR_STOP=1 -U postgres -d "$DB" >/dev/null <<SQL
INSERT INTO users(id,email,password_hash) VALUES ('$USER_ID','preflight@example.test','hash');
INSERT INTO conversations(id,user_id,title) VALUES ('$CONV_ID','$USER_ID','preflight');
INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,status,model,lease_owner,lease_expires_at,lease_heartbeat_at)
VALUES ('$RUN_ID','$CONV_ID','$TURN_ID','preflight-run','$USER_ID','running','synthetic','worker-a',now() + interval '5 minutes',now());
SQL

preflight() {
  BODYSENSE_DEPLOY_ROOT="$TMP_ROOT" BODYSENSE_COMPOSE_PROJECT="$PROJECT" \
    "$TMP_ROOT/scripts/production-deploy-watch.sh" --preflight-only
}

output="$(preflight)"
printf '%s\n' "$output"
grep -q 'DEPLOY_PREFLIGHT=DEFER' <<<"$output"
grep -q 'active_running=1' <<<"$output"

compose exec -T postgres psql -U postgres -d "$DB" -c \
  "UPDATE runs SET status='waiting_user' WHERE id='$RUN_ID';" >/dev/null
output="$(preflight)"
printf '%s\n' "$output"
grep -q 'DEPLOY_PREFLIGHT=READY' <<<"$output"

compose exec -T postgres psql -U postgres -d "$DB" -c \
  "UPDATE runs SET status='running', lease_expires_at=now() - interval '1 minute' WHERE id='$RUN_ID';" >/dev/null
output="$(preflight)"
printf '%s\n' "$output"
grep -q 'DEPLOY_PREFLIGHT=READY' <<<"$output"

compose stop postgres >/dev/null
output="$(preflight)"
printf '%s\n' "$output"
grep -q 'DEPLOY_PREFLIGHT=DEFER' <<<"$output"
grep -q 'unable to verify active Consultation executions' <<<"$output"

echo 'DEPLOY_RUN_PREFLIGHT_INTEGRATION=PASS'
