#!/usr/bin/env bash
set -Eeuo pipefail

CONFIG_FILE=${BODYSENSE_STAGING_CHANNEL_CONFIG:-$HOME/.config/bodysense/staging-channel.env}
STATE_DIR=${BODYSENSE_STAGING_STATE_DIR:-$HOME/.local/state/bodysense}
RUNTIME_ROOT=${BODYSENSE_STAGING_RUNTIME_ROOT:-$HOME/.local/share/bodysense/staging-runtime}
BIN_DIR=${BODYSENSE_STAGING_BIN_DIR:-$HOME/.local/bin}
SYSTEMD_DIR=${BODYSENSE_STAGING_SYSTEMD_DIR:-$HOME/.config/systemd/user}
STATE_FILE="$STATE_DIR/staging-deploy-state"
LOCK_FILE="$STATE_DIR/staging-deploy.lock"
CHECK_ONLY=false
FORCE=false

for arg in "$@"; do
  case "$arg" in
    --check-only) CHECK_ONLY=true ;;
    --force) FORCE=true ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

[[ -s "$CONFIG_FILE" ]] || fail "missing staging channel config: $CONFIG_FILE"
# The channel file contains coordinates only (no credentials). shellcheck disable=SC1090
set -a
source "$CONFIG_FILE"
set +a

REGISTRY=${STAGING_REGISTRY:?set STAGING_REGISTRY in $CONFIG_FILE}
NAMESPACE=${STAGING_NAMESPACE:-bodysense}
CHANNEL_TAG=${STAGING_CHANNEL_TAG:-staging-latest}
SECRET_ENV=${STAGING_SECRET_ENV:-$HOME/.config/bodysense/staging.env}
COMPOSE_PROJECT=${STAGING_COMPOSE_PROJECT:-bodysense-staging}
STAGING_BIND_HOST=${STAGING_BIND_HOST:-127.0.0.1}
STAGING_WEB_PORT=${STAGING_WEB_PORT:-20150}

[[ "$CHANNEL_TAG" == staging-latest ]] || fail "canonical watcher only accepts STAGING_CHANNEL_TAG=staging-latest"
[[ -s "$SECRET_ENV" ]] || fail "missing staging secret env: $SECRET_ENV"
mkdir -p "$STATE_DIR" "$RUNTIME_ROOT" "$BIN_DIR" "$SYSTEMD_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another staging deploy check is already running'; exit 0; }

web_ref="$REGISTRY/$NAMESPACE/bodysense-web:$CHANNEL_TAG"
api_ref="$REGISTRY/$NAMESPACE/bodysense-api:$CHANNEL_TAG"
ai_ref="$REGISTRY/$NAMESPACE/bodysense-ai-service:$CHANNEL_TAG"
runtime_ref="$REGISTRY/$NAMESPACE/bodysense-runtime:$CHANNEL_TAG"

image_revision() {
  docker image inspect "$1" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

container_revision() {
  local service="$1" id image_id
  [[ -s "$RUNTIME_ROOT/docker/docker-compose.staging.yml" ]] || return 0
  id=$(compose ps -q "$service" 2>/dev/null || true)
  [[ -n "$id" ]] || return 0
  image_id=$(docker inspect "$id" --format '{{.Image}}' 2>/dev/null || true)
  [[ -n "$image_id" ]] || return 0
  docker image inspect "$image_id" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

compose() {
  STAGING_WEB_IMAGE="$web_ref" \
  STAGING_API_IMAGE="$api_ref" \
  STAGING_AI_IMAGE="$ai_ref" \
  STAGING_BIND_HOST="$STAGING_BIND_HOST" \
  STAGING_WEB_PORT="$STAGING_WEB_PORT" \
  docker compose \
    -p "$COMPOSE_PROJECT" \
    -f "$RUNTIME_ROOT/docker/docker-compose.staging.yml" \
    --env-file "$SECRET_ENV" \
    "$@"
}

wait_healthy() {
  local service="$1" timeout="${2:-120}" start id status
  start=$(date +%s)
  while :; do
    id=$(compose ps -q "$service" 2>/dev/null || true)
    if [[ -n "$id" ]]; then
      status=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
      [[ "$status" == healthy ]] && return 0
    fi
    if (( $(date +%s) - start >= timeout )); then
      log "$service failed staging health wait"
      [[ -n "${id:-}" ]] && docker logs --tail 100 "$id" 2>&1 || true
      return 1
    fi
    sleep 2
  done
}

assert_container_revision() {
  local service="$1" expected="$2" actual
  actual=$(container_revision "$service")
  [[ "$actual" == "$expected" ]] || {
    log "staging container revision mismatch service=$service expected=$expected actual=${actual:-none}"
    return 1
  }
}

log 'checking ACR staging-latest pointers'
for ref in "$web_ref" "$api_ref" "$ai_ref" "$runtime_ref"; do
  docker pull "$ref" >/dev/null
done

web_revision=$(image_revision "$web_ref")
api_revision=$(image_revision "$api_ref")
ai_revision=$(image_revision "$ai_ref")
runtime_revision=$(image_revision "$runtime_ref")
for pair in \
  "web:$web_revision" \
  "api:$api_revision" \
  "ai:$ai_revision" \
  "runtime:$runtime_revision"; do
  [[ -n "${pair#*:}" ]] || fail "${pair%%:*} staging image has no org.opencontainers.image.revision"
done

if [[ "$web_revision" != "$api_revision" || "$web_revision" != "$ai_revision" || "$web_revision" != "$runtime_revision" ]]; then
  log "staging channel not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision runtime=$runtime_revision"
  exit 0
fi
desired_revision="$web_revision"
managed_revision=$(sed -n 's/^revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)

if $CHECK_ONLY; then
  log "STAGING_CANDIDATE=COHERENT revision=$desired_revision managed=${managed_revision:-none}"
  exit 0
fi

if ! $FORCE && [[ "$managed_revision" == "$desired_revision" ]]; then
  current_web=$(container_revision web)
  current_api=$(container_revision api)
  current_ai=$(container_revision ai-service)
  if [[ "$current_web" == "$desired_revision" && "$current_api" == "$desired_revision" && "$current_ai" == "$desired_revision" ]]; then
    log "already deployed staging revision $desired_revision"
    exit 0
  fi
fi

stage=$(mktemp -d "$STATE_DIR/staging-runtime-next.XXXXXX")
runtime_container=''
cleanup() {
  [[ -z "$runtime_container" ]] || docker rm -f "$runtime_container" >/dev/null 2>&1 || true
  rm -rf "$stage"
}
trap cleanup EXIT
runtime_container=$(docker create "$runtime_ref" /bin/true)
docker cp "$runtime_container:/runtime/staging/." "$stage/"
docker rm "$runtime_container" >/dev/null
runtime_container=''

for required in \
  docker/docker-compose.staging.yml \
  docker/litellm/config.staging.yaml \
  scripts/staging-deploy-watch.sh \
  deploy/systemd/bodysense-staging-deploy-watch.service \
  deploy/systemd/bodysense-staging-deploy-watch.timer; do
  [[ -s "$stage/$required" ]] || fail "candidate runtime missing staging artifact: $required"
done
[[ "$(image_revision "$runtime_ref")" == "$desired_revision" ]] || fail 'runtime image revision changed during extraction'

STAGING_WEB_IMAGE="$web_ref" \
STAGING_API_IMAGE="$api_ref" \
STAGING_AI_IMAGE="$ai_ref" \
STAGING_BIND_HOST="$STAGING_BIND_HOST" \
STAGING_WEB_PORT="$STAGING_WEB_PORT" \
  docker compose \
    -p "$COMPOSE_PROJECT" \
    -f "$stage/docker/docker-compose.staging.yml" \
    --env-file "$SECRET_ENV" \
    config -q

# Runtime configuration is immutable and non-secret. Host secrets remain in the
# external env file and are never copied into the artifact/runtime directory.
rm -rf "$RUNTIME_ROOT.next"
mkdir -p "$RUNTIME_ROOT.next"
cp -a "$stage/." "$RUNTIME_ROOT.next/"
rm -rf "$RUNTIME_ROOT.prev"
if [[ -d "$RUNTIME_ROOT" ]]; then
  mv "$RUNTIME_ROOT" "$RUNTIME_ROOT.prev"
fi
mv "$RUNTIME_ROOT.next" "$RUNTIME_ROOT"

log "deploying coherent staging revision $desired_revision"
compose up -d --no-build postgres redis litellm-gateway
wait_healthy postgres 120 || fail 'staging postgres is unhealthy'
wait_healthy redis 90 || fail 'staging redis is unhealthy'
wait_healthy litellm-gateway 120 || fail 'staging LiteLLM is unhealthy'

# Hide the application ingress while the application release set moves. Staging
# tolerates this short maintenance window in exchange for never serving a mixed
# Web/API/AI revision as the canonical environment.
compose stop web >/dev/null 2>&1 || true
compose up -d --no-deps --no-build api
wait_healthy api 180 || fail 'staging API candidate is unhealthy'
assert_container_revision api "$desired_revision" || fail 'staging API revision check failed'

compose up -d --no-deps --no-build ai-service
wait_healthy ai-service 180 || fail 'staging AI candidate is unhealthy'
assert_container_revision ai-service "$desired_revision" || fail 'staging AI revision check failed'

compose up -d --no-deps --no-build web
wait_healthy web 120 || fail 'staging Web candidate is unhealthy'
assert_container_revision web "$desired_revision" || fail 'staging Web revision check failed'

curl -fsS --max-time 15 "http://${STAGING_BIND_HOST}:${STAGING_WEB_PORT}/api/health" >/dev/null \
  || fail 'staging ingress API health check failed'

# Atomically update the next timer invocation to the watcher/units delivered by
# the exact runtime candidate that just passed staging health.
tmp_watcher="$BIN_DIR/.bodysense-staging-deploy-watch.$$"
cp "$RUNTIME_ROOT/scripts/staging-deploy-watch.sh" "$tmp_watcher"
chmod 0755 "$tmp_watcher"
mv -f "$tmp_watcher" "$BIN_DIR/bodysense-staging-deploy-watch"
install -m 0644 "$RUNTIME_ROOT/deploy/systemd/bodysense-staging-deploy-watch.service" "$SYSTEMD_DIR/bodysense-staging-deploy-watch.service"
install -m 0644 "$RUNTIME_ROOT/deploy/systemd/bodysense-staging-deploy-watch.timer" "$SYSTEMD_DIR/bodysense-staging-deploy-watch.timer"
if command -v systemctl >/dev/null 2>&1; then
  systemctl --user daemon-reload
fi

state_tmp="$STATE_FILE.tmp.$$"
cat > "$state_tmp" <<STATE
revision=$desired_revision
channel=$CHANNEL_TAG
runtime_revision=$runtime_revision
deployed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATE
chmod 0644 "$state_tmp"
mv -f "$state_tmp" "$STATE_FILE"
rm -rf "$RUNTIME_ROOT.prev"
log "STAGING_DEPLOY=PASS revision=$desired_revision"
