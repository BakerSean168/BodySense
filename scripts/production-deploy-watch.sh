#!/usr/bin/env bash
set -Eeuo pipefail

DEPLOY_WATCH_HANDOFF_PROTOCOL=1

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_FILE="$ROOT/.deploy-state"
BLOCK_FILE="$ROOT/.deploy-blocked"
POSTGRES_RESET_STATE_FILE="$ROOT/.postgres18-reset-state"
LOCK_FILE="$ROOT/.deploy.lock"
BACKUP_DIR="$ROOT/backups"
RUNTIME_BACKUP_DIR="$ROOT/runtime-backups"
# Where the host repository of systemd unit symlinks lives.  Production always
# uses /etc/systemd/system; hermetic tests override it via
# BODYSENSE_SYSTEMD_DIR so the rollback's systemd path is exercised without
# mutating the host.
SYSTEMD_DIR="${BODYSENSE_SYSTEMD_DIR:-/etc/systemd/system}"
FORCE=false
CHECK_ONLY=false
PREFLIGHT_ONLY=false
RUNTIME_HANDOFF=false
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
    --runtime-handoff) RUNTIME_HANDOFF=true ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

mkdir -p "$ROOT" "$BACKUP_DIR" "$RUNTIME_BACKUP_DIR"
RUNNING_WATCHER_SHA=$(sha256sum "$0" | awk '{print $1}')
if $RUNTIME_HANDOFF; then
  [ "${BODYSENSE_DEPLOY_HANDOFF_PROTOCOL:-}" = "$DEPLOY_WATCH_HANDOFF_PROTOCOL" ] \
    || fail 'runtime watcher handoff protocol token is missing or incompatible'
  inherited_lock=$(readlink "/proc/$$/fd/9" 2>/dev/null || true)
  [ "$inherited_lock" = "$LOCK_FILE" ] \
    || fail 'runtime watcher handoff is missing the inherited deployment lock'
else
  exec 9>"$LOCK_FILE"
  flock -n 9 || { log 'another deploy check is already running'; exit 0; }
fi

[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"
[ -s "$COMPOSE" ] || fail "missing $COMPOSE"

read_public_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  printf '%s' "${value:-$default}"
}

read_merged_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$SECRET_ENV" | tail -1)
  if [ -z "$value" ]; then
    value=$(read_public_env "$key" "$default")
  fi
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
  install -d -m 0755 "$ROOT/scripts" "$ROOT/deploy/systemd"

  # The DR/operator scripts are part of the managed runtime: restore them exactly
  # as the pre-deploy archive captured them and REMOVE scripts the old runtime
  # did not have, returning the host to the previous state.  The scripts
  # (including production-deploy-watch.sh, which is still executing during a
  # rollback) are replaced ATOMICALLY via a temp file + rename, so the running
  # process keeps reading its already-open file descriptor and the next
  # scheduled run reads the restored file.
  for script in production-deploy-watch.sh offhost-s3.py production-offhost-backup.sh restore-production-backup.sh production-postgres-dr.sh install-production-dr.sh production-postgres18-reset.sh production-capacity-status.sh install-production-capacity.sh; do
    if [ -f "$ROLLBACK_RUNTIME_DIR/scripts/$script" ]; then
      tmp="$ROOT/scripts/.restore-$$-$script.tmp"
      cp -f "$ROLLBACK_RUNTIME_DIR/scripts/$script" "$tmp"
      chmod 0755 "$tmp"
      mv -f "$tmp" "$ROOT/scripts/$script"
    else
      rm -f "$ROOT/scripts/$script"
    fi
  done

  # The PostgreSQL DR, off-host and capacity systemd units are part of the
  # managed runtime: restore (or remove, returning to the previous state) the
  # unit files under $ROOT/deploy/systemd, then re-point the host off-host units
  # in SYSTEMD_DIR (default /etc/systemd/system) so the host never keeps symlinks
  # to the failed revision's units.  With systemd available the loader is
  # refreshed, removed off-host units are explicitly disabled so no timer
  # keep-alive remains, and surviving timers are re-enabled idempotently.
  for unit in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer bodysense-postgres-dr-backup.service bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.service bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.service bodysense-postgres-dr-status.timer bodysense-capacity-status.service bodysense-capacity-status.timer bodysense-capacity-cleanup.service bodysense-capacity-cleanup.timer; do
    if [ -f "$ROLLBACK_RUNTIME_DIR/deploy/systemd/$unit" ]; then
      install -m 0644 "$ROLLBACK_RUNTIME_DIR/deploy/systemd/$unit" "$ROOT/deploy/systemd/$unit"
    else
      rm -f "$ROOT/deploy/systemd/$unit"
    fi
  done
  if command -v systemctl >/dev/null 2>&1; then
    # Legacy AccessKey-based timers are retired globally; a runtime rollback may
    # restore their files for manual compatibility but must never reactivate
    # their scheduler.
    systemctl disable --now bodysense-offhost-backup.timer bodysense-offhost-freshness.timer >/dev/null 2>&1 || true
    systemctl stop bodysense-offhost-backup.service bodysense-offhost-freshness.service >/dev/null 2>&1 || true
    for unit in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer; do
      rm -f "$SYSTEMD_DIR/$unit"
    done
    systemctl daemon-reload
    systemctl reset-failed bodysense-offhost-backup.service bodysense-offhost-freshness.service bodysense-offhost-backup.timer bodysense-offhost-freshness.timer >/dev/null 2>&1 || true
  fi
}

postgres_reset_state_value() {
  local key="$1"
  sed -n "s/^${key}=//p" "$POSTGRES_RESET_STATE_FILE" 2>/dev/null | tail -1 || true
}

postgres_reset_state_status_for_release() {
  local release status
  release=$(postgres_reset_state_value release_revision)
  status=$(postgres_reset_state_value status)
  if [ -n "${desired_revision:-}" ] && [ "$release" = "$desired_revision" ]; then
    printf '%s' "${status:-none}"
  else
    printf '%s' none
  fi
}

rollback_postgres18_reset() {
  [ -n "$ROLLBACK_RUNTIME_DIR" ] || { log 'PostgreSQL reset rollback unavailable: previous runtime archive is missing'; return 1; }
  [ -x "$ROOT/scripts/production-postgres18-reset.sh" ] || { log 'PostgreSQL reset rollback unavailable: reset operator is missing'; return 1; }
  POSTGRES18_RESET_RELEASE_REVISION="$desired_revision" \
  POSTGRES18_RESET_ROLLBACK_COMPOSE="$ROLLBACK_RUNTIME_DIR/docker/docker-compose.prod.yml" \
  POSTGRES18_RESET_ROLLBACK_ENV="$ROLLBACK_RUNTIME_DIR/.env.production" \
    "$ROOT/scripts/production-postgres18-reset.sh" rollback
}

compose_service_exists() {
  local service="$1"
  compose config --services 2>/dev/null | grep -Fxq "$service"
}

rollback_deployment() {
  local current_schema reset_status preserve_runtime=false
  if [ "$ROLLBACK_READY" != true ]; then
    log 'automatic rollback skipped: no complete previous image set was captured'
    return 2
  fi

  reset_status=$(postgres_reset_state_status_for_release)
  case "$reset_status" in
    prepared|cutover_complete)
      rollback_postgres18_reset || return 1
      current_schema="$PREVIOUS_SCHEMA_STATE"
      ;;
    committed)
      # Once the fresh PG18 reset is committed, the discarded legacy volume is
      # gone. Application rollback must keep the PG18 runtime/database boundary.
      preserve_runtime=true
      current_schema=$(db_schema_state)
      log 'PostgreSQL 18 reset is committed; preserving PG18 runtime during application rollback'
      ;;
    *)
      current_schema=$(db_schema_state)
      ;;
  esac

  if [ "$PREVIOUS_SCHEMA_STATE" = unknown ] || [ "$current_schema" = unknown ]; then
    log "automatic rollback skipped: database schema state could not be verified before/after deployment"
    return 3
  fi
  if [ "$current_schema" != "$PREVIOUS_SCHEMA_STATE" ]; then
    log "automatic rollback skipped: database schema changed from $PREVIOUS_SCHEMA_STATE to $current_schema"
    return 3
  fi

  if [ "$preserve_runtime" = true ]; then
    log "database schema unchanged at $current_schema; restoring previous application images on committed PG18 runtime"
  else
    log "database schema unchanged at $current_schema; restoring previous runtime and images"
    restore_runtime || { log 'automatic rollback failed while restoring runtime files'; return 1; }
  fi

  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps litellm-gateway
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy litellm-gateway 120 || return 1
  if compose_service_exists document-service; then
    LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps document-service
    LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy document-service 120 || return 1
  fi
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps ai-service
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy ai-service 120 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps api
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy api 150 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps web
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" wait_healthy web 90 || return 1
  LITELLM_IMAGE="$ROLLBACK_LITELLM_REF" WEB_TAG="$ROLLBACK_TAG" API_TAG="$ROLLBACK_TAG" AI_TAG="$ROLLBACK_TAG" compose up -d --no-deps --force-recreate caddy
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

handoff_to_runtime_watcher_if_needed() {
  local revision="$1" runtime_container handoff_path target_sha handoff_args
  handoff_path="$ROOT/scripts/.deploy-watch-handoff-${revision:0:12}-$$-$(date +%s%N)"
  rm -f "$handoff_path"

  runtime_container=$(docker create "$runtime_ref" /bin/true)
  if ! docker cp "$runtime_container:/runtime/scripts/production-deploy-watch.sh" "$handoff_path"; then
    docker rm -f "$runtime_container" >/dev/null 2>&1 || true
    rm -f "$handoff_path"
    fail 'failed to extract target deploy watcher for runtime handoff'
  fi
  docker rm "$runtime_container" >/dev/null
  [ -s "$handoff_path" ] || { rm -f "$handoff_path"; fail 'target runtime deploy watcher is empty'; }
  chmod 0755 "$handoff_path"
  target_sha=$(sha256sum "$handoff_path" | awk '{print $1}')
  if [ "$target_sha" = "$RUNNING_WATCHER_SHA" ]; then
    rm -f "$handoff_path"
    return 0
  fi
  if ! grep -Eq '^DEPLOY_WATCH_HANDOFF_PROTOCOL=1$' "$handoff_path"; then
    rm -f "$handoff_path"
    fail 'target runtime deploy watcher changed without compatible handoff protocol 1'
  fi

  log "deploy watcher changed running=$RUNNING_WATCHER_SHA target=$target_sha; handing off before backup/schema/service changes"
  handoff_args=(--runtime-handoff)
  $FORCE && handoff_args+=(--force)
  exec env \
    BODYSENSE_DEPLOY_HANDOFF_PROTOCOL="$DEPLOY_WATCH_HANDOFF_PROTOCOL" \
    BODYSENSE_DEPLOY_HANDOFF_EXECUTABLE="$handoff_path" \
    "$handoff_path" "${handoff_args[@]}"
}

cleanup_handoff_executable() {
  local path="${BODYSENSE_DEPLOY_HANDOFF_EXECUTABLE:-}"
  [ -n "$path" ] || return 0
  case "$path" in
    "$ROOT"/scripts/.deploy-watch-handoff-*) rm -f "$path" ;;
  esac
}

sync_runtime() {
  local revision="$1" stage="$ROOT/.runtime-next" old_runtime_revision runtime_container archive stage_postgres_major

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
  [ -s "$stage/scripts/offhost-s3.py" ] || fail 'runtime bundle missing off-host S3 client'
  [ -s "$stage/scripts/production-offhost-backup.sh" ] || fail 'runtime bundle missing off-host backup script'
  [ -s "$stage/scripts/restore-production-backup.sh" ] || fail 'runtime bundle missing off-host restore script'
  for unit in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer; do
    [ -s "$stage/deploy/systemd/$unit" ] || fail "runtime bundle missing $unit"
  done
  # The PostgreSQL DR and capacity runners are packaged into the same runtime
  # bundle by the Dockerfile; when present they must be complete.  Absence is
  # tolerated so a DR-only runtime bundle (for example in hermetic off-host
  # rollback tests) can still be synchronized.
  if [ -e "$stage/scripts/production-postgres-dr.sh" ] || [ -e "$stage/scripts/install-production-dr.sh" ]; then
    [ -s "$stage/scripts/production-postgres-dr.sh" ] || fail 'runtime bundle missing PostgreSQL DR runner'
    [ -s "$stage/scripts/install-production-dr.sh" ] || fail 'runtime bundle missing PostgreSQL DR installer'
  fi
  if [ -e "$stage/scripts/production-capacity-status.sh" ] || [ -e "$stage/scripts/install-production-capacity.sh" ]; then
    [ -s "$stage/scripts/production-capacity-status.sh" ] || fail 'runtime bundle missing capacity status runner'
    [ -s "$stage/scripts/install-production-capacity.sh" ] || fail 'runtime bundle missing capacity installer'
  fi
  stage_postgres_major=$(sed -n 's/^POSTGRES_MAJOR=//p' "$stage/.env.production" | tail -1)
  if [ -n "$stage_postgres_major" ]; then
    [ -s "$stage/scripts/production-postgres18-reset.sh" ] \
      || fail 'runtime declares POSTGRES_MAJOR but bundle is missing PostgreSQL 18 reset operator'
  elif [ -e "$stage/scripts/production-postgres18-reset.sh" ]; then
    [ -s "$stage/scripts/production-postgres18-reset.sh" ] || fail 'runtime bundle has an empty PostgreSQL 18 reset operator'
  fi
  [ "$(image_revision "$runtime_ref")" = "$revision" ] || fail 'runtime bundle revision mismatch'

  docker compose -p "$COMPOSE_PROJECT" -f "$stage/docker/docker-compose.prod.yml" \
    --env-file "$stage/.env.production" --env-file "$SECRET_ENV" config -q

  old_runtime_revision=$(sed -n 's/^runtime_revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)
  [ -n "$old_runtime_revision" ] || old_runtime_revision=pre-managed
  archive="$RUNTIME_BACKUP_DIR/${old_runtime_revision}-$(date -u +%Y%m%d-%H%M%S)"
  mkdir -p "$archive/docker/litellm" "$archive/scripts" "$archive/deploy/systemd"
  cp -f "$PUBLIC_ENV" "$archive/.env.production" 2>/dev/null || true
  cp -f "$COMPOSE" "$archive/docker/docker-compose.prod.yml" 2>/dev/null || true
  cp -f "$ROOT/docker/Caddyfile" "$archive/docker/Caddyfile" 2>/dev/null || true
  cp -f "$ROOT/docker/litellm/config.yaml" "$archive/docker/litellm/config.yaml" 2>/dev/null || true
  # The DR/operator scripts and the systemd units are part of the managed
  # runtime, so a failed deployment can roll back the WHOLE runtime -- not just
  # the stack files -- to the exact previous state.
  cp -f "$ROOT/scripts/production-deploy-watch.sh" "$archive/scripts/production-deploy-watch.sh" 2>/dev/null || true
  for script in offhost-s3.py production-offhost-backup.sh restore-production-backup.sh production-postgres-dr.sh install-production-dr.sh production-postgres18-reset.sh production-capacity-status.sh install-production-capacity.sh; do
    cp -f "$ROOT/scripts/$script" "$archive/scripts/$script" 2>/dev/null || true
  done
  for unit in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer bodysense-postgres-dr-backup.service bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.service bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.service bodysense-postgres-dr-status.timer bodysense-capacity-status.service bodysense-capacity-status.timer bodysense-capacity-cleanup.service bodysense-capacity-cleanup.timer; do
    cp -f "$ROOT/deploy/systemd/$unit" "$archive/deploy/systemd/$unit" 2>/dev/null || true
  done
  ROLLBACK_RUNTIME_DIR="$archive"

  install -d -m 0755 "$ROOT/docker/litellm" "$ROOT/scripts" "$ROOT/deploy/systemd"
  RUNTIME_CHANGED=true
  install -m 0644 "$stage/.env.production" "$PUBLIC_ENV"
  install -m 0644 "$stage/docker/docker-compose.prod.yml" "$COMPOSE"
  install -m 0644 "$stage/docker/Caddyfile" "$ROOT/docker/Caddyfile"
  [ -f "$stage/docker/litellm/config.yaml" ] && install -m 0644 "$stage/docker/litellm/config.yaml" "$ROOT/docker/litellm/config.yaml"
  # production-deploy-watch.sh installs ITSELF atomically. A deployment whose
  # target watcher differs is handed off BEFORE backup/schema/service changes,
  # so by the time sync_runtime runs here the active deployer already implements
  # the target release's deployment contract.
  dw_tmp="$ROOT/scripts/.deploy-watch-$$.tmp"
  cp -f "$stage/scripts/production-deploy-watch.sh" "$dw_tmp"
  chmod 0755 "$dw_tmp"
  mv -f "$dw_tmp" "$ROOT/scripts/production-deploy-watch.sh"
  for script in offhost-s3.py production-offhost-backup.sh restore-production-backup.sh; do
    install -m 0755 "$stage/scripts/$script" "$ROOT/scripts/$script"
  done
  for script in production-postgres-dr.sh install-production-dr.sh production-postgres18-reset.sh production-capacity-status.sh install-production-capacity.sh; do
    if [ -e "$stage/scripts/$script" ]; then
      install -m 0755 "$stage/scripts/$script" "$ROOT/scripts/$script"
    else
      rm -f "$ROOT/scripts/$script"
    fi
  done
  install_offhost_units() {
    # Legacy AccessKey-based off-host artifacts are preserved in the managed
    # runtime for manual restore compatibility, but their automatic timers are
    # retired. The supported production DR path is production-postgres-dr.sh
    # with ECS RAM Role + private OSS and is gated by DR_ENABLED.
    install -d -m 0755 "$ROOT/deploy/systemd"
    install -m 0644 "$stage/deploy/systemd/bodysense-offhost-backup.service" "$ROOT/deploy/systemd/bodysense-offhost-backup.service"
    install -m 0644 "$stage/deploy/systemd/bodysense-offhost-backup.timer" "$ROOT/deploy/systemd/bodysense-offhost-backup.timer"
    install -m 0644 "$stage/deploy/systemd/bodysense-offhost-freshness.service" "$ROOT/deploy/systemd/bodysense-offhost-freshness.service"
    install -m 0644 "$stage/deploy/systemd/bodysense-offhost-freshness.timer" "$ROOT/deploy/systemd/bodysense-offhost-freshness.timer"
  }
  retire_legacy_offhost_units() {
    local unit
    for unit in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer; do
      rm -f "$SYSTEMD_DIR/$unit"
    done
    if command -v systemctl >/dev/null 2>&1; then
      systemctl disable --now bodysense-offhost-backup.timer bodysense-offhost-freshness.timer >/dev/null 2>&1 || true
      systemctl stop bodysense-offhost-backup.service bodysense-offhost-freshness.service >/dev/null 2>&1 || true
      systemctl daemon-reload
      systemctl reset-failed bodysense-offhost-backup.service bodysense-offhost-freshness.service bodysense-offhost-backup.timer bodysense-offhost-freshness.timer >/dev/null 2>&1 || true
    fi
  }
  install_offhost_units
  retire_legacy_offhost_units
  for unit in bodysense-postgres-dr-backup.service bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.service bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.service bodysense-postgres-dr-status.timer bodysense-capacity-status.service bodysense-capacity-status.timer bodysense-capacity-cleanup.service bodysense-capacity-cleanup.timer; do
    if [ -e "$stage/deploy/systemd/$unit" ]; then
      install -m 0644 "$stage/deploy/systemd/$unit" "$ROOT/deploy/systemd/$unit"
    else
      rm -f "$ROOT/deploy/systemd/$unit"
    fi
  done
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
current_document=$(container_revision document-service)
current_ai=$(container_revision ai-service)
managed_revision=$(sed -n 's/^revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)

if $CHECK_ONLY; then
  preflight=READY
  deploy_run_preflight || preflight=DEFER
  log "coherent candidate revision=$desired_revision runtime=$runtime_revision current_web=${current_web:-none} current_api=${current_api:-none} current_document=${current_document:-none} current_ai=${current_ai:-none} managed=${managed_revision:-none} run_preflight=$preflight"
  exit 0
fi

if ! $FORCE && [ "$AUTO_DEPLOY" != true ]; then
  log "candidate $desired_revision is coherent; AUTO_DEPLOY_ENABLED=$AUTO_DEPLOY"
  exit 0
fi
if ! $FORCE && [ "$desired_revision" = "$current_web" ] && [ "$desired_revision" = "$current_api" ] && [ "$desired_revision" = "$current_document" ] && [ "$desired_revision" = "$current_ai" ] && [ "$desired_revision" = "$managed_revision" ]; then
  log "already deployed revision $desired_revision"
  exit 0
fi

if ! deploy_run_preflight; then
  log "deployment deferred revision=$desired_revision; watcher will retry on its next schedule"
  exit 0
fi

# The runtime bundle owns the deployment contract. If that contract changed,
# transfer the still-side-effect-free transaction to the target watcher before
# creating a database backup, changing schema, or touching application services.
handoff_to_runtime_watcher_if_needed "$desired_revision"

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
  cleanup_handoff_executable
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

if [ -x "$ROOT/scripts/install-production-capacity.sh" ]; then
  log 'ensuring bounded host swap exists before memory-capped service changes'
  "$ROOT/scripts/install-production-capacity.sh" --swap-only >/dev/null || fail 'capacity swap preflight failed'
fi

if [ "$(read_merged_env DR_ENABLED false)" = true ] && [ -x "$ROOT/scripts/production-postgres-dr.sh" ]; then
  log 'off-host DR gate enabled; creating a verified OSS backup before production service/schema changes'
  "$ROOT/scripts/production-postgres-dr.sh" backup >/dev/null || fail 'off-host PostgreSQL backup gate failed'
fi

TARGET_POSTGRES_MAJOR=$(read_public_env POSTGRES_MAJOR "")
if [ -n "$TARGET_POSTGRES_MAJOR" ]; then
  [ -x "$ROOT/scripts/production-postgres18-reset.sh" ] || fail 'runtime declares POSTGRES_MAJOR but PostgreSQL 18 reset operator is missing'
  POSTGRES18_RESET_RELEASE_REVISION="$desired_revision" "$ROOT/scripts/production-postgres18-reset.sh" cutover     || fail 'PostgreSQL 18 reset failed'
fi

compose pull litellm-gateway >/dev/null
compose up -d --no-deps litellm-gateway
wait_healthy litellm-gateway 120 || fail 'litellm-gateway deployment failed'

deploy_document_service() {
  compose up -d --no-deps document-service
  wait_healthy document-service 120 || fail 'document-service deployment failed'
  assert_container_revision document-service "$desired_revision" || fail 'document-service revision verification failed'
}

deploy_ai_service() {
  compose up -d --no-deps ai-service
  wait_healthy ai-service 120 || fail 'ai-service deployment failed'
  assert_container_revision ai-service "$desired_revision" || fail 'ai-service revision verification failed'
}

deploy_api_service() {
  compose up -d --no-deps api
  wait_healthy api 150 || fail 'api deployment failed'
  assert_container_revision api "$desired_revision" || fail 'api revision verification failed'
}

reset_status=$(postgres_reset_state_status_for_release)
if [ "$reset_status" = cutover_complete ]; then
  # A fresh PG18 database has no vector extension yet. The Go API owns schema
  # migrations (migration 10 creates vector), while AI registers the vector type
  # during its FastAPI lifespan. Bootstrap schema first, with Caddy still down.
  log 'fresh PostgreSQL 18 detected; bootstrapping API migrations before AI service'
  deploy_api_service
  deploy_document_service
  deploy_ai_service
else
  # Existing databases are already migrated. Bring the bounded document runtime
  # up before the API can enqueue report work, then preserve the established AI
  # before externally reachable API order.
  deploy_document_service
  deploy_ai_service
  deploy_api_service
fi

compose up -d --no-deps web
wait_healthy web 90 || fail 'web deployment failed'
assert_container_revision web "$desired_revision" || fail 'web revision verification failed'

if [ "$(postgres_reset_state_status_for_release)" = cutover_complete ]; then
  POSTGRES18_RESET_RELEASE_REVISION="$desired_revision" "$ROOT/scripts/production-postgres18-reset.sh" commit     || fail 'PostgreSQL 18 reset commit failed'
fi

# Caddy is deliberately exposed only after the fresh PostgreSQL 18 reset is committed.
# Before this point the legacy database can still be restored if the fresh PG18 service itself fails health checks.
compose up -d --no-deps --force-recreate caddy
compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 || true

curl -fsS --max-time 15 "https://${APP_DOMAIN}/api/health" >/dev/null || fail 'external API health check failed'

cat > "$STATE_FILE" <<STATE
revision=$desired_revision
runtime_revision=$desired_revision
runtime_source=acr
deployed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE

if [ -x "$ROOT/scripts/install-production-dr.sh" ]; then
  # The installer is bootstrap-safe: it installs the unit files on first
  # deployment, but only enables their timers when DR_ENABLED=true.
  "$ROOT/scripts/install-production-dr.sh" >/dev/null
fi
if [ -x "$ROOT/scripts/install-production-capacity.sh" ]; then
  "$ROOT/scripts/install-production-capacity.sh" >/dev/null
fi
chmod 0644 "$STATE_FILE"
rm -f "$BLOCK_FILE"
deploy_started=false
trap - EXIT
cleanup_rollback_tags
find "$BACKUP_DIR" -type f -name 'bodysense-pre-*.dump*' -mtime +14 -delete 2>/dev/null || true
find "$RUNTIME_BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -mtime +14 -exec rm -rf {} + 2>/dev/null || true
cleanup_handoff_executable
log "deployment successful revision=$desired_revision"
