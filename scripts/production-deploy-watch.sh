#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_FILE="$ROOT/.deploy-state"
BLOCK_FILE="$ROOT/.deploy-blocked"
LOCK_FILE="$ROOT/.deploy.lock"
BACKUP_DIR="$ROOT/backups"
RUNTIME_BACKUP_DIR="$ROOT/runtime-backups"
FORCE=false
CHECK_ONLY=false
PREFLIGHT_ONLY=false
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"
ROLLBACK_READY=false
ROLLBACK_TAG=""
ROLLBACK_WEB_REF=""
ROLLBACK_API_REF=""
ROLLBACK_AI_REF=""
ROLLBACK_LITELLM_REF=""
ROLLBACK_RUNTIME_DIR=""
RUNTIME_CHANGED=false
PREVIOUS_SCHEMA_STATE="unknown"
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --check-only) CHECK_ONLY=true ;;
    --preflight-only) PREFLIGHT_ONLY=true ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

mkdir -p "$ROOT" "$BACKUP_DIR" "$RUNTIME_BACKUP_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another deploy check is already running'; exit 0; }

[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"
[ -s "$COMPOSE" ] || fail "missing $COMPOSE"

read_public_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  printf '%s' "${value:-$default}"
}

REGISTRY=$(read_public_env REGISTRY)
NAMESPACE=$(read_public_env ACR_NAMESPACE bodysense)
WEB_TAG=$(read_public_env WEB_TAG prod-latest)
API_TAG=$(read_public_env API_TAG prod-latest)
AI_TAG=$(read_public_env AI_TAG prod-latest)
RUNTIME_TAG=$(read_public_env RUNTIME_TAG prod-latest)
APP_DOMAIN=$(read_public_env APP_DOMAIN body.bakersean.top)
AUTO_DEPLOY=$(read_public_env AUTO_DEPLOY_ENABLED true)
DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)

web_ref="$REGISTRY/$NAMESPACE/bodysense-web:$WEB_TAG"
api_ref="$REGISTRY/$NAMESPACE/bodysense-api:$API_TAG"
ai_ref="$REGISTRY/$NAMESPACE/bodysense-ai-service:$AI_TAG"
runtime_ref="$REGISTRY/$NAMESPACE/bodysense-runtime:$RUNTIME_TAG"

compose() {
  docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE" --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" "$@"
}

image_revision() {
  docker image inspect "$1" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

container_image_id() {
  local id
  id=$(compose ps -q "$1" 2>/dev/null || true)
  if [ -n "$id" ]; then
    docker inspect "$id" --format '{{.Image}}' 2>/dev/null || true
  fi
}

container_revision() {
  local image_id
  image_id=$(container_image_id "$1")
  if [ -n "$image_id" ]; then
    docker image inspect "$image_id" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
  fi
}

active_execution_count() {
  local table_exists lease_columns count
  if ! table_exists=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT to_regclass('public.runs') IS NOT NULL;" 2>/dev/null); then
    printf '%s' unknown
    return 0
  fi
  if [ "$table_exists" != t ]; then
    printf '%s' 0
    return 0
  fi

  if ! lease_columns=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='runs' AND column_name IN ('lease_owner','lease_expires_at');" 2>/dev/null); then
    printf '%s' unknown
    return 0
  fi

  if [ "$lease_columns" = 2 ]; then
    if ! count=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
      "SELECT count(*) FROM runs WHERE status='running' AND lease_owner IS NOT NULL AND btrim(lease_owner) <> '' AND lease_expires_at > now();" 2>/dev/null); then
      printf '%s' unknown
      return 0
    fi
  else
    # Bootstrap compatibility for the first release that introduces leases. On
    # a pre-lease schema every running row is conservatively treated as active.
    if ! count=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
      "SELECT count(*) FROM runs WHERE status='running';" 2>/dev/null); then
      printf '%s' unknown
      return 0
    fi
  fi
  printf '%s' "${count:-unknown}"
}

deploy_run_preflight() {
  local active_count
  active_count=$(active_execution_count)
  case "$active_count" in
    ''|*[!0-9]*)
      log 'deploy preflight DEFER: unable to verify active Consultation executions; automated deploy will retry later'
      return 1
      ;;
  esac
  if [ "$active_count" -gt 0 ]; then
    log "deploy preflight DEFER: active_running=$active_count with valid execution lease; waiting_user does not block deploy"
    return 1
  fi
  log 'deploy preflight READY: active_running=0 (waiting_user is intentionally ignored)'
  return 0
}

if $PREFLIGHT_ONLY; then
  if deploy_run_preflight; then
    log 'DEPLOY_PREFLIGHT=READY'
  else
    log 'DEPLOY_PREFLIGHT=DEFER'
  fi
  exit 0
fi

db_schema_state() {
  local exists value
  if ! exists=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT to_regclass('public.schema_migrations') IS NOT NULL;" 2>/dev/null); then
    printf '%s' unknown
    return 0
  fi
  if [ "$exists" != t ]; then
    printf '%s' uninitialized
    return 0
  fi
  if ! value=$(compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;" \
    2>/dev/null); then
    printf '%s' unknown
    return 0
  fi
  printf '%s' "${value:-uninitialized}"
}

prepare_rollback_images() {
  local web_id api_id ai_id litellm_id stamp base_revision
  web_id=$(container_image_id web)
  api_id=$(container_image_id api)
  ai_id=$(container_image_id ai-service)
  litellm_id=$(container_image_id litellm-gateway)
  if [ -z "$web_id" ] || [ -z "$api_id" ] || [ -z "$ai_id" ] || [ -z "$litellm_id" ]; then
    log 'automatic rollback unavailable: previous application/LiteLLM image set is incomplete'
    return 0
  fi

  base_revision="${managed_revision:-pre-managed}"
  stamp=$(date -u +%Y%m%d-%H%M%S)
  ROLLBACK_TAG="rollback-${base_revision:0:12}-${stamp}"
  ROLLBACK_WEB_REF="$REGISTRY/$NAMESPACE/bodysense-web:$ROLLBACK_TAG"
  ROLLBACK_API_REF="$REGISTRY/$NAMESPACE/bodysense-api:$ROLLBACK_TAG"
  ROLLBACK_AI_REF="$REGISTRY/$NAMESPACE/bodysense-ai-service:$ROLLBACK_TAG"
  ROLLBACK_LITELLM_REF="$REGISTRY/$NAMESPACE/litellm:$ROLLBACK_TAG"

  docker tag "$web_id" "$ROLLBACK_WEB_REF"
  docker tag "$api_id" "$ROLLBACK_API_REF"
  docker tag "$ai_id" "$ROLLBACK_AI_REF"
  docker tag "$litellm_id" "$ROLLBACK_LITELLM_REF"
  ROLLBACK_READY=true
}

restore_runtime() {
  [ "$RUNTIME_CHANGED" = true ] || return 0
  [ -n "$ROLLBACK_RUNTIME_DIR" ] || return 1
  [ -s "$ROLLBACK_RUNTIME_DIR/.env.production" ] || return 1
  [ -s "$ROLLBACK_RUNTIME_DIR/docker/docker-compose.prod.yml" ] || return 1
  [ -s "$ROLLBACK_RUNTIME_DIR/docker/Caddyfile" ] || return 1

  install -m 0644 "$ROLLBACK_RUNTIME_DIR/.env.production" "$PUBLIC_ENV"
  install -m 0644 "$ROLLBACK_RUNTIME_DIR/docker/docker-compose.prod.yml" "$COMPOSE"
  install -m 0644 "$ROLLBACK_RUNTIME_DIR/docker/Caddyfile" "$ROOT/docker/Caddyfile"
  if [ -f "$ROLLBACK_RUNTIME_DIR/docker/litellm/config.yaml" ]; then
    install -d -m 0755 "$ROOT/docker/litellm"
    install -m 0644 "$ROLLBACK_RUNTIME_DIR/docker/litellm/config.yaml" "$ROOT/docker/litellm/config.yaml"
  else
    rm -f "$ROOT/docker/litellm/config.yaml"
  fi
}

rollback_deployment() {
  local current_schema
  if [ "$ROLLBACK_READY" != true ]; then
    log 'automatic rollback skipped: no complete previous image set was captured'
    return 2
  fi

  current_schema=$(db_schema_state)
  if [ "$PREVIOUS_SCHEMA_STATE" = unknown ] || [ "$current_schema" = unknown ]; then
    log "automatic rollback skipped: database schema state could not be verified before/after deployment"
    return 3
  fi
  if [ "$current_schema" != "$PREVIOUS_SCHEMA_STATE" ]; then
    log "automatic rollback skipped: database schema changed from $PREVIOUS_SCHEMA_STATE to $current_schema"
    return 3
  fi

  log "database schema unchanged at $current_schema; restoring previous runtime and images"
  restore_runtime || { log 'automatic rollback failed while restoring runtime files'; return 1; }

  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps litellm-gateway
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy litellm-gateway 120 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps ai-service
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy ai-service 120 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps api
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy api 150 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps web
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy web 90 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps caddy
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 || true
  curl -fsS --max-time 15 "https://${APP_DOMAIN}/api/health" >/dev/null || return 1
  log "automatic rollback restored managed revision ${managed_revision:-unknown}"
  return 0
}

cleanup_rollback_tags() {
  local ref
  for ref in "$ROLLBACK_WEB_REF" "$ROLLBACK_API_REF" "$ROLLBACK_AI_REF" "$ROLLBACK_LITELLM_REF"; do
    if [ -n "$ref" ]; then
      docker rmi "$ref" >/dev/null 2>&1 || true
    fi
  done
}

assert_container_revision() {
  local service="$1" expected="$2" actual
  actual=$(container_revision "$service")
  if [ "$actual" != "$expected" ]; then
    log "container revision mismatch service=$service expected=$expected actual=${actual:-none}"
    return 1
  fi
}

wait_healthy() {
  local service="$1" timeout="${2:-120}" id status start
  start=$(date +%s)
  while :; do
    id=$(compose ps -q "$service" 2>/dev/null || true)
    if [ -n "$id" ]; then
      status=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
      [ "$status" = healthy ] && return 0
      [ "$status" = running ] && [ "$service" = caddy ] && return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      log "$service failed health wait"
      if [ -n "${id:-}" ]; then
        docker logs --tail 120 "$id" 2>&1 || true
      fi
      return 1
    fi
    sleep 2
  done
}

sync_runtime() {
  local revision="$1" stage="$ROOT/.runtime-next" old_runtime_revision runtime_container archive

  rm -rf "$stage"
  mkdir -p "$stage"
  runtime_container=$(docker create "$runtime_ref" /bin/true)
  if ! docker cp "$runtime_container:/runtime/." "$stage/"; then
    docker rm -f "$runtime_container" >/dev/null 2>&1 || true
    fail 'failed to extract ACR runtime bundle'
  fi
  docker rm "$runtime_container" >/dev/null

  [ -s "$stage/.env.production" ] || fail 'runtime bundle missing .env.production'
  [ -s "$stage/docker/docker-compose.prod.yml" ] || fail 'runtime bundle missing production Compose'
  [ -s "$stage/docker/Caddyfile" ] || fail 'runtime bundle missing Caddyfile'
  [ -s "$stage/scripts/production-deploy-watch.sh" ] || fail 'runtime bundle missing deploy watcher'
  [ "$(image_revision "$runtime_ref")" = "$revision" ] || fail 'runtime bundle revision mismatch'

  docker compose -p "$COMPOSE_PROJECT" -f "$stage/docker/docker-compose.prod.yml" \
    --env-file "$stage/.env.production" --env-file "$SECRET_ENV" config -q

  old_runtime_revision=$(sed -n 's/^runtime_revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)
  [ -n "$old_runtime_revision" ] || old_runtime_revision=pre-managed
  archive="$RUNTIME_BACKUP_DIR/${old_runtime_revision}-$(date -u +%Y%m%d-%H%M%S)"
  mkdir -p "$archive/docker/litellm"
  cp -f "$PUBLIC_ENV" "$archive/.env.production" 2>/dev/null || true
  cp -f "$COMPOSE" "$archive/docker/docker-compose.prod.yml" 2>/dev/null || true
  cp -f "$ROOT/docker/Caddyfile" "$archive/docker/Caddyfile" 2>/dev/null || true
  cp -f "$ROOT/docker/litellm/config.yaml" "$archive/docker/litellm/config.yaml" 2>/dev/null || true
  ROLLBACK_RUNTIME_DIR="$archive"

  install -d -m 0755 "$ROOT/docker/litellm" "$ROOT/scripts"
  RUNTIME_CHANGED=true
  install -m 0644 "$stage/.env.production" "$PUBLIC_ENV"
  install -m 0644 "$stage/docker/docker-compose.prod.yml" "$COMPOSE"
  install -m 0644 "$stage/docker/Caddyfile" "$ROOT/docker/Caddyfile"
  [ -f "$stage/docker/litellm/config.yaml" ] && install -m 0644 "$stage/docker/litellm/config.yaml" "$ROOT/docker/litellm/config.yaml"
  install -m 0755 "$stage/scripts/production-deploy-watch.sh" "$ROOT/scripts/production-deploy-watch.sh"
}

[ -n "$REGISTRY" ] || fail 'REGISTRY is empty'

log 'checking ACR production pointers'
docker pull "$web_ref" >/dev/null
docker pull "$api_ref" >/dev/null
docker pull "$ai_ref" >/dev/null
docker pull "$runtime_ref" >/dev/null

web_revision=$(image_revision "$web_ref")
api_revision=$(image_revision "$api_ref")
ai_revision=$(image_revision "$ai_ref")
runtime_revision=$(image_revision "$runtime_ref")
[ -n "$web_revision" ] || fail 'web image is missing org.opencontainers.image.revision'
[ -n "$runtime_revision" ] || fail 'runtime image is missing org.opencontainers.image.revision'
[ "$web_revision" = "$api_revision" ] || { log "release not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision runtime=$runtime_revision"; exit 0; }
[ "$web_revision" = "$ai_revision" ] || { log "release not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision runtime=$runtime_revision"; exit 0; }
[ "$web_revision" = "$runtime_revision" ] || { log "release not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision runtime=$runtime_revision"; exit 0; }
desired_revision="$web_revision"
blocked_revision=$(sed -n 's/^revision=//p' "$BLOCK_FILE" 2>/dev/null | tail -1 || true)
if ! $FORCE && [ "$blocked_revision" = "$desired_revision" ]; then
  log "revision $desired_revision is blocked after a previous failed deployment; wait for a new release or use --force after investigation"
  exit 0
fi

current_web=$(container_revision web)
current_api=$(container_revision api)
current_ai=$(container_revision ai-service)
managed_revision=$(sed -n 's/^revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)

if $CHECK_ONLY; then
  preflight=READY
  deploy_run_preflight || preflight=DEFER
  log "coherent candidate revision=$desired_revision runtime=$runtime_revision current_web=${current_web:-none} current_api=${current_api:-none} current_ai=${current_ai:-none} managed=${managed_revision:-none} run_preflight=$preflight"
  exit 0
fi

if ! $FORCE && [ "$AUTO_DEPLOY" != true ]; then
  log "candidate $desired_revision is coherent; AUTO_DEPLOY_ENABLED=$AUTO_DEPLOY"
  exit 0
fi
if ! $FORCE && [ "$desired_revision" = "$current_web" ] && [ "$desired_revision" = "$current_api" ] && [ "$desired_revision" = "$current_ai" ] && [ "$desired_revision" = "$managed_revision" ]; then
  log "already deployed revision $desired_revision"
  exit 0
fi

if ! deploy_run_preflight; then
  log "deployment deferred revision=$desired_revision; watcher will retry on its next schedule"
  exit 0
fi

log "deploying coherent revision $desired_revision"
# Legacy Watchtower updated containers independently and can race this coherent
# release transaction. Remove it before the managed cutover on upgraded hosts.
docker rm -f docker-watchtower-1 >/dev/null 2>&1 || true

PREVIOUS_SCHEMA_STATE=$(db_schema_state)
prepare_rollback_images

# Back up the stable, currently-running database before any runtime files or
# services are changed.
backup="$BACKUP_DIR/bodysense-pre-${desired_revision:0:12}-$(date -u +%Y%m%d-%H%M%S).dump"
compose exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$backup"
[ -s "$backup" ] || fail 'database backup is empty'
sha256sum "$backup" > "$backup.sha256"
postgres_container=$(compose ps -q postgres)
[ -n "$postgres_container" ] || fail 'postgres container is not running for backup validation'
backup_container_path="/tmp/$(basename "$backup")"
docker cp "$backup" "$postgres_container:$backup_container_path" >/dev/null
if ! compose exec -T postgres pg_restore --list "$backup_container_path" >/dev/null; then
  compose exec -T postgres rm -f "$backup_container_path" >/dev/null 2>&1 || true
  fail 'database backup failed pg_restore archive validation'
fi
compose exec -T postgres rm -f "$backup_container_path" >/dev/null
log "database backup created and validated: $(basename "$backup") schema=$PREVIOUS_SCHEMA_STATE"

deploy_started=true
on_exit() {
  status=$?
  trap - EXIT
  if [ "$status" -ne 0 ] && [ "${deploy_started:-false}" = true ]; then
    rollback_status=skipped
    if rollback_deployment; then
      rollback_status=restored
    else
      rollback_code=$?
      case "$rollback_code" in
        2) rollback_status=unavailable ;;
        3) rollback_status=skipped-schema-changed ;;
        *) rollback_status=failed ;;
      esac
    fi
    current_schema=$(db_schema_state)
    cat > "$BLOCK_FILE" <<BLOCK
revision=$desired_revision
failed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
rollback=$rollback_status
schema_before=$PREVIOUS_SCHEMA_STATE
schema_after=$current_schema
backup=$(basename "$backup")
BLOCK
    log "revision $desired_revision marked blocked after deployment failure rollback=$rollback_status"
  fi
  exit "$status"
}
trap on_exit EXIT
checked_registry="$REGISTRY"
checked_namespace="$NAMESPACE"
checked_web_tag="$WEB_TAG"
checked_api_tag="$API_TAG"
checked_ai_tag="$AI_TAG"
checked_runtime_tag="$RUNTIME_TAG"
checked_db_user="$DB_USER"
checked_db_name="$DB_NAME"
sync_runtime "$desired_revision"

# Re-evaluate variables after the runtime bundle is synchronized. Deployment pointer
# or database identity changes require a separate bootstrap; one application release
# may not silently change the coordinates used for the release transaction.
REGISTRY=$(read_public_env REGISTRY)
NAMESPACE=$(read_public_env ACR_NAMESPACE bodysense)
WEB_TAG=$(read_public_env WEB_TAG prod-latest)
API_TAG=$(read_public_env API_TAG prod-latest)
AI_TAG=$(read_public_env AI_TAG prod-latest)
RUNTIME_TAG=$(read_public_env RUNTIME_TAG prod-latest)
DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)
[ "$REGISTRY" = "$checked_registry" ] || fail 'runtime bundle changed REGISTRY during deployment'
[ "$NAMESPACE" = "$checked_namespace" ] || fail 'runtime bundle changed ACR_NAMESPACE during deployment'
[ "$WEB_TAG" = "$checked_web_tag" ] || fail 'runtime bundle changed WEB_TAG during deployment'
[ "$API_TAG" = "$checked_api_tag" ] || fail 'runtime bundle changed API_TAG during deployment'
[ "$AI_TAG" = "$checked_ai_tag" ] || fail 'runtime bundle changed AI_TAG during deployment'
[ "$RUNTIME_TAG" = "$checked_runtime_tag" ] || fail 'runtime bundle changed RUNTIME_TAG during deployment'
[ "$DB_USER" = "$checked_db_user" ] || fail 'runtime bundle changed DB_USER during deployment'
[ "$DB_NAME" = "$checked_db_name" ] || fail 'runtime bundle changed DB_NAME during deployment'
APP_DOMAIN=$(read_public_env APP_DOMAIN body.bakersean.top)

compose pull litellm-gateway >/dev/null
compose up -d --no-deps litellm-gateway
wait_healthy litellm-gateway 120 || fail 'litellm-gateway deployment failed'

compose up -d --no-deps ai-service
wait_healthy ai-service 120 || fail 'ai-service deployment failed'
assert_container_revision ai-service "$desired_revision" || fail 'ai-service revision verification failed'

compose up -d --no-deps api
wait_healthy api 150 || fail 'api deployment failed'
assert_container_revision api "$desired_revision" || fail 'api revision verification failed'

compose up -d --no-deps web
wait_healthy web 90 || fail 'web deployment failed'
assert_container_revision web "$desired_revision" || fail 'web revision verification failed'

# Caddy is infrastructure, but reload its config if the runtime bundle changed.
compose up -d --no-deps caddy
compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 || true

curl -fsS --max-time 15 "https://${APP_DOMAIN}/api/health" >/dev/null || fail 'external API health check failed'

cat > "$STATE_FILE" <<STATE
revision=$desired_revision
runtime_revision=$desired_revision
runtime_source=acr
deployed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE
chmod 0644 "$STATE_FILE"
rm -f "$BLOCK_FILE"
deploy_started=false
trap - EXIT
cleanup_rollback_tags
find "$BACKUP_DIR" -type f -name 'bodysense-pre-*.dump*' -mtime +14 -delete 2>/dev/null || true
find "$RUNTIME_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -mtime +14 -exec rm -rf {} + 2>/dev/null || true
log "deployment successful revision=$desired_revision"
