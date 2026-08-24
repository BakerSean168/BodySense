#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_DIR="$ROOT/dr-state"
LOCK_FILE="$ROOT/.postgres-dr.lock"
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"
COMMAND="${1:-}"

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2; }
fail() { log "ERROR: $*"; exit 1; }

case "$COMMAND" in
  backup|status|restore-drill) ;;
  *) fail 'usage: production-postgres-dr.sh <backup|status|restore-drill>' ;;
esac

[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"
[ -s "$COMPOSE" ] || fail "missing $COMPOSE"
mkdir -p "$STATE_DIR"
chmod 700 "$STATE_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another PostgreSQL DR operation is running'; exit 0; }

read_env_value() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$SECRET_ENV" | tail -1)
  if [ -z "$value" ]; then
    value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  fi
  printf '%s' "${value:-$default}"
}

DR_ENABLED=$(read_env_value DR_ENABLED false)
if [ "$DR_ENABLED" != true ]; then
  log 'PostgreSQL off-host DR is disabled; configure a private OSS target and ECS RAM role first'
  exit 0
fi

compose=(
  docker compose -p "$COMPOSE_PROJECT"
  -f "$COMPOSE"
  --env-file "$PUBLIC_ENV"
  --env-file "$SECRET_ENV"
)

api_container=$("${compose[@]}" ps -q api 2>/dev/null || true)
[ -n "$api_container" ] || fail 'api container is not running; cannot bind DR operation to a deployed revision'
api_image_id=$(docker inspect "$api_container" --format '{{.Image}}')
revision=$(docker image inspect "$api_image_id" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')
[ -n "$revision" ] || fail 'running api image has no revision label'

log "starting PostgreSQL DR command=$COMMAND revision=$revision"
result=$("${compose[@]}" --profile ops run --rm --no-deps \
  -e "DR_RELEASE_REVISION=$revision" \
  dr "$COMMAND")
[ -n "$result" ] || fail 'DR manager returned empty result'

state_name=${COMMAND//-/_}
tmp=$(mktemp "$STATE_DIR/.last-${state_name}.XXXXXX")
printf '%s\n' "$result" > "$tmp"
chmod 600 "$tmp"
mv -f "$tmp" "$STATE_DIR/last-${state_name}.json"
printf '%s\n' "$result"
log "PostgreSQL DR command=$COMMAND completed and state was persisted"
