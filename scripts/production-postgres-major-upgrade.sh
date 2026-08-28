#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
BACKUP_DIR="$ROOT/backups"
STATE_FILE="$ROOT/.postgres-major-upgrade-state"
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"
COMMAND="${1:-cutover}"
RELEASE_REVISION="${POSTGRES_MAJOR_RELEASE_REVISION:-unknown}"

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

service_container_for() {
  local service="$1"
  docker ps -aq \
    --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" \
    --filter "label=com.docker.compose.service=$service" | head -1
}

server_major() {
  local id="$1" version_num
  version_num=$(docker exec "$id" psql -U "$DB_USER" -d "$DB_NAME" -Atc 'show server_version_num')
  printf '%s' $(( version_num / 10000 ))
}

schema_state() {
  local id="$1" exists value
  exists=$(docker exec "$id" psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT to_regclass('public.schema_migrations') IS NOT NULL;")
  if [ "$exists" != t ]; then
    printf '%s' uninitialized
    return 0
  fi
  value=$(docker exec "$id" psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;")
  printf '%s' "${value:-uninitialized}"
}

table_count_digest() {
  local id="$1"
  docker exec -i "$id" psql -U "$DB_USER" -d "$DB_NAME" -Atq <<'SQL' \
    | sha256sum | awk '{print $1}'
SELECT format('SELECT %L || count(*) FROM %I.%I;', tablename || '=', schemaname, tablename)
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY tablename
\gexec
SQL
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
      if [ -n "${id:-}" ]; then
        docker logs --tail 120 "$id" 2>&1 || true
      fi
      return 1
    fi
    sleep 2
  done
}

write_state() {
  local status="$1" source_major="$2" target_major="$3" source_volume="$4" target_volume="$5" backup="$6" schema="$7" digest="$8"
  local tmp="$STATE_FILE.tmp.$$"
  cat > "$tmp" <<STATE
status=$status
release_revision=$RELEASE_REVISION
from_major=$source_major
to_major=$target_major
source_volume=$source_volume
target_volume=$target_volume
backup=$backup
schema=$schema
count_digest=$digest
updated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE
  chmod 0600 "$tmp"
  mv -f "$tmp" "$STATE_FILE"
}

[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"
[ -s "$COMPOSE" ] || fail "missing $COMPOSE"
mkdir -p "$BACKUP_DIR"

DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)
TARGET_MAJOR=$(read_public_env POSTGRES_MAJOR 18)
TARGET_VOLUME=$(read_public_env POSTGRES_DATA_VOLUME bodysense-postgres-pg18)
case "$TARGET_MAJOR" in ''|*[!0-9]*) fail "invalid POSTGRES_MAJOR=$TARGET_MAJOR" ;; esac
[ -n "$TARGET_VOLUME" ] || fail 'POSTGRES_DATA_VOLUME is empty'

case "$COMMAND" in
  status)
    id=$(service_container)
    [ -n "$id" ] || fail 'production postgres container is not present'
    printf 'POSTGRES_MAJOR=%s\n' "$(server_major "$id")"
    printf 'POSTGRES_SCHEMA=%s\n' "$(schema_state "$id")"
    printf 'UPGRADE_STATUS=%s\n' "$(state_value status)"
    printf 'UPGRADE_RELEASE=%s\n' "$(state_value release_revision)"
    exit 0
    ;;
  commit)
    id=$(service_container)
    [ -n "$id" ] || fail 'cannot commit PostgreSQL major upgrade without a running postgres container'
    current_major=$(server_major "$id")
    [ "$current_major" = "$TARGET_MAJOR" ] || fail "cannot commit PostgreSQL major upgrade: running major=$current_major target=$TARGET_MAJOR"
    status=$(state_value status)
    state_release=$(state_value release_revision)
    if [ "$status" = committed ] && [ "$state_release" = "$RELEASE_REVISION" ]; then
      log "PostgreSQL major upgrade already committed release=$RELEASE_REVISION major=$TARGET_MAJOR"
      exit 0
    fi
    [ "$status" = cutover_complete ] || fail "cannot commit PostgreSQL major upgrade from state=${status:-none}"
    [ "$state_release" = "$RELEASE_REVISION" ] || fail "PostgreSQL major upgrade state belongs to release=${state_release:-none}, expected=$RELEASE_REVISION"
    write_state committed "$(state_value from_major)" "$TARGET_MAJOR" "$(state_value source_volume)" "$TARGET_VOLUME" \
      "$(state_value backup)" "$(state_value schema)" "$(state_value count_digest)"
    log "PostgreSQL major upgrade committed release=$RELEASE_REVISION major=$TARGET_MAJOR"
    exit 0
    ;;
  rollback)
    status=$(state_value status)
    state_release=$(state_value release_revision)
    case "$status" in prepared|cutover_complete) ;; *) fail "cannot roll back PostgreSQL major upgrade from state=${status:-none}" ;; esac
    [ "$state_release" = "$RELEASE_REVISION" ] || fail "PostgreSQL major upgrade state belongs to release=${state_release:-none}, expected=$RELEASE_REVISION"
    rollback_compose_file="${POSTGRES_MAJOR_ROLLBACK_COMPOSE:?set POSTGRES_MAJOR_ROLLBACK_COMPOSE}"
    rollback_env_file="${POSTGRES_MAJOR_ROLLBACK_ENV:?set POSTGRES_MAJOR_ROLLBACK_ENV}"
    expected_schema="${POSTGRES_MAJOR_EXPECTED_SCHEMA:?set POSTGRES_MAJOR_EXPECTED_SCHEMA}"
    [ -s "$rollback_compose_file" ] || fail "missing rollback Compose $rollback_compose_file"
    [ -s "$rollback_env_file" ] || fail "missing rollback env $rollback_env_file"
    source_major=$(state_value from_major)
    source_volume=$(state_value source_volume)
    backup_name=$(state_value backup)
    saved_schema=$(state_value schema)
    saved_digest=$(state_value count_digest)

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
      if [ $(( $(date +%s) - start )) -ge 120 ]; then
        [ -z "${restored_id:-}" ] || docker logs --tail 120 "$restored_id" 2>&1 || true
        fail 'previous PostgreSQL did not become healthy during major-upgrade rollback'
      fi
      sleep 2
    done
    restored_num=$(docker exec "$restored_id" psql -U "$DB_USER" -d "$DB_NAME" -Atc 'show server_version_num')
    restored_major=$(( restored_num / 10000 ))
    [ "$restored_major" = "$source_major" ] || fail "rollback restored wrong PostgreSQL major=$restored_major expected=$source_major"
    restored_schema=$(schema_state "$restored_id")
    [ "$restored_schema" = "$expected_schema" ] || fail "rollback schema mismatch restored=$restored_schema expected=$expected_schema"
    restored_mount=$(mount_name_for_destination "$restored_id" /var/lib/postgresql/data)
    [ "$restored_mount" = "$source_volume" ] || fail "rollback mounted source volume=${restored_mount:-none} expected=$source_volume"

    if [ -n "$TARGET_VOLUME" ] && [ "$TARGET_VOLUME" != "$source_volume" ] && docker volume inspect "$TARGET_VOLUME" >/dev/null 2>&1; then
      docker volume rm "$TARGET_VOLUME" >/dev/null 2>&1 || log "warning: rolled-back PG18 target volume $TARGET_VOLUME remains for manual inspection"
    fi
    write_state rolled_back "$source_major" "$TARGET_MAJOR" "$source_volume" "$TARGET_VOLUME" \
      "$backup_name" "$saved_schema" "$saved_digest"
    log "PostgreSQL major rollback restored major=$source_major schema=$restored_schema source_volume=$source_volume"
    exit 0
    ;;
  cutover) ;;
  *) fail 'usage: production-postgres-major-upgrade.sh <cutover|commit|rollback|status>' ;;
esac

source_id=$(service_container)
if [ -z "$source_id" ]; then
  log 'PostgreSQL major cutover skipped: no existing production postgres container (fresh bootstrap)'
  exit 0
fi
current_major=$(server_major "$source_id")
if [ "$current_major" = "$TARGET_MAJOR" ]; then
  log "PostgreSQL major cutover not required: already on major=$TARGET_MAJOR"
  exit 0
fi
if [ "$current_major" != 16 ] || [ "$TARGET_MAJOR" != 18 ]; then
  fail "automatic PostgreSQL major cutover supports only 16 -> 18; current=$current_major target=$TARGET_MAJOR"
fi

source_volume=$(mount_name_for_destination "$source_id" /var/lib/postgresql/data)
[ -n "$source_volume" ] || fail 'unable to identify PG16 source volume mounted at /var/lib/postgresql/data'
[ "$source_volume" != "$TARGET_VOLUME" ] || fail 'source and target PostgreSQL volumes must be different'
if docker volume inspect "$TARGET_VOLUME" >/dev/null 2>&1; then
  fail "target PostgreSQL volume $TARGET_VOLUME already exists; refusing to overwrite unknown data"
fi

backup="$BACKUP_DIR/bodysense-pg${current_major}-to-pg${TARGET_MAJOR}-${RELEASE_REVISION:0:12}-$(date -u +%Y%m%d-%H%M%S).dump"

# Stop every externally reachable or database-writing application component
# before the final migration dump. Caddy stays down until the watcher has
# validated the restored PG18 database and internally healthy application stack.
for service in caddy api ai-service; do
  id=$(service_container_for "$service")
  [ -z "$id" ] || docker stop "$id" >/dev/null
 done

# Re-resolve the source after writers are quiesced and take the authoritative
# migration snapshot. The pre-deploy watcher backup remains an independent
# rollback artifact; this dump is the exact source for the major cutover.
source_id=$(service_container)
[ -n "$source_id" ] || fail 'source postgres disappeared before final major-upgrade dump'
source_schema=$(schema_state "$source_id")
[ "$source_schema" != unknown ] || fail 'unable to read source schema state after quiescing writers'
source_digest=$(table_count_digest "$source_id")
[ -n "$source_digest" ] || fail 'unable to compute source table-count digest after quiescing writers'
docker exec "$source_id" pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$backup"
[ -s "$backup" ] || fail 'PostgreSQL major-upgrade dump is empty'
sha256sum "$backup" > "$backup.sha256"
backup_in_container="/tmp/$(basename "$backup")"
docker cp "$backup" "$source_id:$backup_in_container" >/dev/null
docker exec "$source_id" pg_restore --list "$backup_in_container" >/dev/null
docker exec "$source_id" rm -f "$backup_in_container" >/dev/null 2>&1 || true

write_state prepared "$current_major" "$TARGET_MAJOR" "$source_volume" "$TARGET_VOLUME" \
  "$(basename "$backup")" "$source_schema" "$source_digest"
log "PostgreSQL major cutover prepared from=$current_major to=$TARGET_MAJOR schema=$source_schema source_volume=$source_volume target_volume=$TARGET_VOLUME"

compose pull postgres >/dev/null
# Recreate the Compose postgres service using the PG18 image and its independent
# versioned volume. The old PG16 volume is intentionally left untouched.
docker stop "$source_id" >/dev/null 2>&1 || true
compose up -d --no-deps postgres
if ! target_id=$(wait_postgres 150); then
  fail 'PG18 postgres failed health wait after service recreation'
fi
new_major=$(server_major "$target_id")
[ "$new_major" = "$TARGET_MAJOR" ] || fail "postgres service started wrong major=$new_major expected=$TARGET_MAJOR"
target_mount=$(mount_name_for_destination "$target_id" /var/lib/postgresql)
[ "$target_mount" = "$TARGET_VOLUME" ] \
  || fail "PG18 postgres is not mounted on expected target volume=$TARGET_VOLUME actual=${target_mount:-none}"

docker cp "$backup" "$target_id:$backup_in_container" >/dev/null
docker exec "$target_id" pg_restore --list "$backup_in_container" >/dev/null
docker exec "$target_id" pg_restore -U "$DB_USER" -d "$DB_NAME" --no-owner --no-privileges --exit-on-error "$backup_in_container" >/dev/null
docker exec "$target_id" rm -f "$backup_in_container" >/dev/null 2>&1 || true

target_schema=$(schema_state "$target_id")
[ "$target_schema" = "$source_schema" ] \
  || fail "PG18 restored schema mismatch source=$source_schema target=$target_schema"
target_digest=$(table_count_digest "$target_id")
[ "$target_digest" = "$source_digest" ] \
  || fail "PG18 restored public-table count digest mismatch source=$source_digest target=$target_digest"
vector_version=$(docker exec "$target_id" psql -U "$DB_USER" -d "$DB_NAME" -Atc \
  "SELECT extversion FROM pg_extension WHERE extname='vector';")
[ -n "$vector_version" ] || fail 'PG18 restored database is missing vector extension'

write_state cutover_complete "$current_major" "$TARGET_MAJOR" "$source_volume" "$TARGET_VOLUME" \
  "$(basename "$backup")" "$target_schema" "$target_digest"
log "PostgreSQL major cutover complete from=$current_major to=$TARGET_MAJOR schema=$target_schema vector=$vector_version count_digest=$target_digest"
