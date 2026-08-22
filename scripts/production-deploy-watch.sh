#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
REPO_DIR="$ROOT/.release-src"
REPO_URL="${BODYSENSE_REPO_URL:-https://github.com/T1moooo/BodySense.git}"
STATE_FILE="$ROOT/.deploy-state"
BLOCK_FILE="$ROOT/.deploy-blocked"
LOCK_FILE="$ROOT/.deploy.lock"
BACKUP_DIR="$ROOT/backups"
RUNTIME_BACKUP_DIR="$ROOT/runtime-backups"
FORCE=false
CHECK_ONLY=false
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --check-only) CHECK_ONLY=true ;;
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
APP_DOMAIN=$(read_public_env APP_DOMAIN body.bakersean.top)
AUTO_DEPLOY=$(read_public_env AUTO_DEPLOY_ENABLED true)
DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)

[ -n "$REGISTRY" ] || fail 'REGISTRY is empty'

web_ref="$REGISTRY/$NAMESPACE/bodysense-web:$WEB_TAG"
api_ref="$REGISTRY/$NAMESPACE/bodysense-api:$API_TAG"
ai_ref="$REGISTRY/$NAMESPACE/bodysense-ai-service:$AI_TAG"

compose() {
  docker compose -p docker -f "$COMPOSE" --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" "$@"
}

image_revision() {
  docker image inspect "$1" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

container_image_id() {
  local id
  id=$(compose ps -q "$1" 2>/dev/null || true)
  [ -n "$id" ] && docker inspect "$id" --format '{{.Image}}' 2>/dev/null || true
}

container_revision() {
  local image_id
  image_id=$(container_image_id "$1")
  [ -n "$image_id" ] && docker image inspect "$image_id" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
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
      [ -n "${id:-}" ] && docker logs --tail 120 "$id" 2>&1 || true
      return 1
    fi
    sleep 2
  done
}

sync_runtime() {
  local revision="$1" stage="$ROOT/.runtime-next" old_runtime_revision
  if [ ! -d "$REPO_DIR/.git" ]; then
    rm -rf "$REPO_DIR"
    git clone --filter=blob:none --no-checkout "$REPO_URL" "$REPO_DIR" >/dev/null
  fi
  git -C "$REPO_DIR" fetch --quiet --depth=1 origin "$revision"
  git -C "$REPO_DIR" checkout --quiet --detach FETCH_HEAD
  [ "$(git -C "$REPO_DIR" rev-parse HEAD)" = "$revision" ] || fail 'runtime checkout revision mismatch'

  rm -rf "$stage"
  mkdir -p "$stage/docker/litellm" "$stage/scripts"
  install -m 0644 "$REPO_DIR/.env.production" "$stage/.env.production"
  install -m 0644 "$REPO_DIR/docker/docker-compose.prod.yml" "$stage/docker/docker-compose.prod.yml"
  install -m 0644 "$REPO_DIR/docker/Caddyfile" "$stage/docker/Caddyfile"
  [ -f "$REPO_DIR/docker/litellm/config.yaml" ] && install -m 0644 "$REPO_DIR/docker/litellm/config.yaml" "$stage/docker/litellm/config.yaml"
  install -m 0755 "$REPO_DIR/scripts/production-deploy-watch.sh" "$stage/scripts/production-deploy-watch.sh"

  docker compose -p docker -f "$stage/docker/docker-compose.prod.yml" \
    --env-file "$stage/.env.production" --env-file "$SECRET_ENV" config -q

  old_runtime_revision=$(sed -n 's/^runtime_revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)
  [ -n "$old_runtime_revision" ] || old_runtime_revision=pre-managed
  local archive="$RUNTIME_BACKUP_DIR/${old_runtime_revision}-$(date -u +%Y%m%d-%H%M%S)"
  mkdir -p "$archive/docker"
  cp -f "$PUBLIC_ENV" "$archive/.env.production" 2>/dev/null || true
  cp -f "$COMPOSE" "$archive/docker/docker-compose.prod.yml" 2>/dev/null || true
  cp -f "$ROOT/docker/Caddyfile" "$archive/docker/Caddyfile" 2>/dev/null || true

  install -d -m 0755 "$ROOT/docker/litellm" "$ROOT/scripts"
  install -m 0644 "$stage/.env.production" "$PUBLIC_ENV"
  install -m 0644 "$stage/docker/docker-compose.prod.yml" "$COMPOSE"
  install -m 0644 "$stage/docker/Caddyfile" "$ROOT/docker/Caddyfile"
  [ -f "$stage/docker/litellm/config.yaml" ] && install -m 0644 "$stage/docker/litellm/config.yaml" "$ROOT/docker/litellm/config.yaml"
  install -m 0755 "$stage/scripts/production-deploy-watch.sh" "$ROOT/scripts/production-deploy-watch.sh"
}

log 'checking ACR production pointers'
docker pull "$web_ref" >/dev/null
docker pull "$api_ref" >/dev/null
docker pull "$ai_ref" >/dev/null

web_revision=$(image_revision "$web_ref")
api_revision=$(image_revision "$api_ref")
ai_revision=$(image_revision "$ai_ref")
[ -n "$web_revision" ] || fail 'web image is missing org.opencontainers.image.revision'
[ "$web_revision" = "$api_revision" ] || { log "release not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision"; exit 0; }
[ "$web_revision" = "$ai_revision" ] || { log "release not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision"; exit 0; }
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
  log "coherent candidate revision=$desired_revision current_web=${current_web:-none} current_api=${current_api:-none} current_ai=${current_ai:-none} managed=${managed_revision:-none}"
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

log "deploying coherent revision $desired_revision"
# Legacy Watchtower updated containers independently and can race this coherent
# release transaction. Remove it before the managed cutover on upgraded hosts.
docker rm -f docker-watchtower-1 >/dev/null 2>&1 || true
deploy_started=true
on_exit() {
  status=$?
  if [ "$status" -ne 0 ] && [ "${deploy_started:-false}" = true ]; then
    cat > "$BLOCK_FILE" <<BLOCK
revision=$desired_revision
failed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
BLOCK
    log "revision $desired_revision marked blocked after deployment failure"
  fi
}
trap on_exit EXIT
checked_registry="$REGISTRY"
checked_namespace="$NAMESPACE"
checked_web_tag="$WEB_TAG"
checked_api_tag="$API_TAG"
checked_ai_tag="$AI_TAG"
sync_runtime "$desired_revision"

# Re-evaluate variables after the runtime bundle is synchronized. Deployment pointer
# changes require a separate bootstrap; one release may not change the coordinates
# that were used to establish image coherence.
REGISTRY=$(read_public_env REGISTRY)
NAMESPACE=$(read_public_env ACR_NAMESPACE bodysense)
WEB_TAG=$(read_public_env WEB_TAG prod-latest)
API_TAG=$(read_public_env API_TAG prod-latest)
AI_TAG=$(read_public_env AI_TAG prod-latest)
[ "$REGISTRY" = "$checked_registry" ] || fail 'runtime bundle changed REGISTRY during deployment'
[ "$NAMESPACE" = "$checked_namespace" ] || fail 'runtime bundle changed ACR_NAMESPACE during deployment'
[ "$WEB_TAG" = "$checked_web_tag" ] || fail 'runtime bundle changed WEB_TAG during deployment'
[ "$API_TAG" = "$checked_api_tag" ] || fail 'runtime bundle changed API_TAG during deployment'
[ "$AI_TAG" = "$checked_ai_tag" ] || fail 'runtime bundle changed AI_TAG during deployment'
APP_DOMAIN=$(read_public_env APP_DOMAIN body.bakersean.top)
DB_USER=$(read_public_env DB_USER bodysense)
DB_NAME=$(read_public_env DB_NAME bodysense)

backup="$BACKUP_DIR/bodysense-pre-${desired_revision:0:12}-$(date -u +%Y%m%d-%H%M%S).dump"
compose exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$backup"
[ -s "$backup" ] || fail 'database backup is empty'
sha256sum "$backup" > "$backup.sha256"
log "database backup created: $(basename "$backup")"

compose pull litellm-gateway >/dev/null
compose up -d --no-deps litellm-gateway
wait_healthy litellm-gateway 120 || fail 'litellm-gateway deployment failed'

compose up -d --no-deps ai-service
wait_healthy ai-service 120 || fail 'ai-service deployment failed'

compose up -d --no-deps api
wait_healthy api 150 || fail 'api deployment failed'

compose up -d --no-deps web
wait_healthy web 90 || fail 'web deployment failed'

# Caddy is infrastructure, but reload its config if the runtime bundle changed.
compose up -d --no-deps caddy
compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 || true

curl -fsS --max-time 15 "https://${APP_DOMAIN}/api/health" >/dev/null || fail 'external API health check failed'

cat > "$STATE_FILE" <<STATE
revision=$desired_revision
runtime_revision=$desired_revision
deployed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE
chmod 0644 "$STATE_FILE"
rm -f "$BLOCK_FILE"
deploy_started=false
trap - EXIT
find "$BACKUP_DIR" -type f -name 'bodysense-pre-*.dump*' -mtime +14 -delete 2>/dev/null || true
log "deployment successful revision=$desired_revision"
