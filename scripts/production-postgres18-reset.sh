#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_FILE="$ROOT/.postgres18-reset-state"
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"
COMMAND="${1:-cutover}"
RELEASE_REVISION="${POSTGRES18_RESET_RELEASE_REVISION:-unknown}"

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

read_public_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  printf '%s' "${value:-$default}"
}

state_value() {
  local key="$1"
  sed -n "s/^${key}=//p" "$STATE_FILE" 2>/dev/null | tail -1 || true
}

compose() {
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE" --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" "$@"
}

service_container() {
  docker ps -aq \
    --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" \
    --filter 'label=com.docker.compose.service=postgres' | head -1
}

server_major() {
  local id="$1" version_num
  version_num=$(docker exec "$id" psql -U "$DB_USER" -d "$DB_NAME" -Atc 'show server_version_num')
  printf '%s' $(( version_num / 10000 ))
}

mount_name_for_destination() {
  local id="$1" destination="$2"
  docker inspect "$id" --format "{{range .Mounts}}{{if eq .Destination \"$destination\"}}{{.Name}}{{end}}{{end}}"
}

wait_postgres() {
  local timeout="${1:-120}" id status start
  start=$(date +%s)
  while :; do
    id=$(service_container)
    if [ -n "$id" ]; then
      status=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
      if [ "$status" = healthy ]; then
        printf '%s' "$id"
        return 0
      fi
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      [ -z "${id:-}" ] || docker logs --tail 120 "$id" 2>&1 || true
      return 1
    fi
    sleep 2
  done
}

write_state() {
  local status="$1" source_major="$2" source_volume="$3" target_volume="$4"
  local tmp="$STATE_FILE.tmp.$$"
  cat > "$tmp" <<STATE
status=$status
release_revision=$RELEASE_REVISION
source_major=$source_major
target_major=$TARGET_MAJOR
source_volume=$source_volume
target_volume=$target_volume
updated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE
  chmod 0600 "$tmp"
  mv -f "$tmp" "$STATE_FILE"
}

[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"
[ -s "$COMPOSE" ] || fail "missing $COMPOSE"

DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)
TARGET_MAJOR=$(read_public_env POSTGRES_MAJOR 18)
TARGET_VOLUME=$(read_public_env POSTGRES_DATA_VOLUME bodysense-postgres-pg18)
[ "$TARGET_MAJOR" = 18 ] || fail "BodySense production supports PostgreSQL 18 only; target=$TARGET_MAJOR"
[ -n "$TARGET_VOLUME" ] || fail 'POSTGRES_DATA_VOLUME is empty'

case "$COMMAND" in
  status)
    id=$(service_container)
    [ -n "$id" ] || fail 'production postgres container is not present'
    printf 'POSTGRES_MAJOR=%s\n' "$(server_major "$id")"
    printf 'RESET_STATUS=%s\n' "$(state_value status)"
    printf 'RESET_RELEASE=%s\n' "$(state_value release_revision)"
    exit 0
    ;;
  commit)
    id=$(service_container)
    [ -n "$id" ] || fail 'cannot commit PostgreSQL 18 reset without a running postgres container'
    [ "$(server_major "$id")" = 18 ] || fail 'cannot commit PostgreSQL 18 reset while running a non-PG18 server'
    status=$(state_value status)
    state_release=$(state_value release_revision)
    if [ "$status" = committed ] && [ "$state_release" = "$RELEASE_REVISION" ]; then
      log "PostgreSQL 18 reset already committed release=$RELEASE_REVISION"
      exit 0
    fi
    [ "$status" = cutover_complete ] || fail "cannot commit PostgreSQL 18 reset from state=${status:-none}"
    [ "$state_release" = "$RELEASE_REVISION" ] || fail "reset state belongs to release=${state_release:-none}, expected=$RELEASE_REVISION"
    source_major=$(state_value source_major)
    source_volume=$(state_value source_volume)
    if [ -n "$source_volume" ] && [ "$source_volume" != "$TARGET_VOLUME" ]; then
      docker volume rm "$source_volume" >/dev/null || fail "failed to delete discarded PostgreSQL volume $source_volume"
      log "discarded legacy PostgreSQL volume $source_volume"
    fi
    write_state committed "$source_major" "$source_volume" "$TARGET_VOLUME"
    log "PostgreSQL 18 reset committed release=$RELEASE_REVISION target_volume=$TARGET_VOLUME"
    exit 0
    ;;
  rollback)
    status=$(state_value status)
    state_release=$(state_value release_revision)
    case "$status" in prepared|cutover_complete) ;; *) fail "cannot roll back PostgreSQL 18 reset from state=${status:-none}" ;; esac
    [ "$state_release" = "$RELEASE_REVISION" ] || fail "reset state belongs to release=${state_release:-none}, expected=$RELEASE_REVISION"
    rollback_compose_file="${POSTGRES18_RESET_ROLLBACK_COMPOSE:?set POSTGRES18_RESET_ROLLBACK_COMPOSE}"
    rollback_env_file="${POSTGRES18_RESET_ROLLBACK_ENV:?set POSTGRES18_RESET_ROLLBACK_ENV}"
    source_major=$(state_value source_major)
    source_volume=$(state_value source_volume)
    current_id=$(service_container)
    [ -z "$current_id" ] || docker stop "$current_id" >/dev/null 2>&1 || true
    rollback_compose() {
      docker compose -p "$COMPOSE_PROJECT" -f "$rollback_compose_file" --env-file "$rollback_env_file" --env-file "$SECRET_ENV" "$@"
    }
    rollback_compose up -d --no-deps postgres
    start=$(date +%s)
    while :; do
      restored_id=$(rollback_compose ps -q postgres 2>/dev/null || true)
      if [ -n "$restored_id" ]; then
        health=$(docker inspect "$restored_id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
        [ "$health" = healthy ] && break
      fi
      [ $(( $(date +%s) - start )) -lt 120 ] || fail 'legacy PostgreSQL failed health wait during reset rollback'
      sleep 2
    done
    [ "$(server_major "$restored_id")" = "$source_major" ] || fail 'reset rollback restored the wrong PostgreSQL major'
    restored_mount=$(mount_name_for_destination "$restored_id" /var/lib/postgresql/data)
    [ "$restored_mount" = "$source_volume" ] || fail "reset rollback mounted ${restored_mount:-none}, expected=$source_volume"
    if docker volume inspect "$TARGET_VOLUME" >/dev/null 2>&1; then
      docker volume rm "$TARGET_VOLUME" >/dev/null 2>&1 || log "warning: PG18 target volume $TARGET_VOLUME remains for inspection"
    fi
    write_state rolled_back "$source_major" "$source_volume" "$TARGET_VOLUME"
    log "PostgreSQL 18 reset rolled back before commit"
    exit 0
    ;;
  cutover) ;;
  *) fail 'usage: production-postgres18-reset.sh <cutover|commit|rollback|status>' ;;
esac

source_id=$(service_container)
if [ -z "$source_id" ]; then
  log 'no existing postgres container; starting fresh PostgreSQL 18'
  compose pull postgres >/dev/null
  compose up -d --no-deps postgres
  target_id=$(wait_postgres 150) || fail 'fresh PostgreSQL 18 failed health wait'
  [ "$(server_major "$target_id")" = 18 ] || fail 'fresh production database did not start on PostgreSQL 18'
  write_state committed none none "$TARGET_VOLUME"
  exit 0
fi

current_major=$(server_major "$source_id")
if [ "$current_major" = 18 ]; then
  log 'PostgreSQL 18 reset not required: production is already on PostgreSQL 18'
  exit 0
fi
[ "$current_major" = 16 ] || fail "unexpected legacy PostgreSQL major=$current_major; refusing destructive reset"

source_volume=$(mount_name_for_destination "$source_id" /var/lib/postgresql/data)
[ -n "$source_volume" ] || fail 'unable to identify legacy PostgreSQL source volume'
[ "$source_volume" != "$TARGET_VOLUME" ] || fail 'legacy and PG18 target volumes must be different'
if docker volume inspect "$TARGET_VOLUME" >/dev/null 2>&1; then
  fail "PG18 target volume $TARGET_VOLUME already exists before reset; refusing to overwrite unknown state"
fi

# There is no production data retention requirement for the legacy database.
# Quiesce externally reachable writers, switch to a fresh PG18 volume, and keep
# the old volume only until the new application stack passes its health gates.
for service in caddy api ai-service; do
  id=$(docker ps -aq \
    --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" \
    --filter "label=com.docker.compose.service=$service" | head -1)
  [ -z "$id" ] || docker stop "$id" >/dev/null
 done

write_state prepared "$current_major" "$source_volume" "$TARGET_VOLUME"
compose pull postgres >/dev/null
docker stop "$source_id" >/dev/null 2>&1 || true
compose up -d --no-deps postgres
target_id=$(wait_postgres 150) || fail 'PostgreSQL 18 failed health wait after legacy reset'
[ "$(server_major "$target_id")" = 18 ] || fail 'production postgres did not switch to major 18'
target_mount=$(mount_name_for_destination "$target_id" /var/lib/postgresql)
[ "$target_mount" = "$TARGET_VOLUME" ] || fail "PG18 mounted ${target_mount:-none}, expected=$TARGET_VOLUME"
write_state cutover_complete "$current_major" "$source_volume" "$TARGET_VOLUME"
log "PostgreSQL 18 fresh reset complete; legacy volume will be deleted after application health gates"
