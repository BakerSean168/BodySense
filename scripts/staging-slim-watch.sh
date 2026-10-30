#!/usr/bin/env bash
# Slim BodySense staging watcher for a host that also runs production (root, system timer).
# Installed by scripts/install-staging-slim-watch.sh; see docs/runbooks/staging-slim.md.
#
# Same coherent staging-latest channel and runtime artifact as scripts/staging-deploy-watch.sh,
# plus the deploy/staging-slim/docker-compose.staging.slim.yml override, a memory-headroom
# guard, and no self-update (the canonical watcher would overwrite this copy). It never
# touches the production compose project, except attaching staging ai-service to the
# production network by container name so it can call the production litellm-gateway.
#
# Staging containers use restart: "no", so this watcher is the only thing that starts them,
# including after a host reboot (timer OnBootSec): the stack comes back only when the
# headroom guard allows a deploy.
set -Eeuo pipefail

ROOT=${BODYSENSE_STAGING_SLIM_ROOT:-/opt/bodysense-staging}
SECRET_ENV=$ROOT/.env.staging.local
OVERRIDE=$ROOT/docker-compose.staging.slim.yml
RUNTIME_ROOT=$ROOT/runtime
STATE_DIR=$ROOT/state
STATE_FILE=$STATE_DIR/staging-deploy-state
LOCK_FILE=$STATE_DIR/staging-deploy.lock
REGISTRY=${STAGING_REGISTRY:-crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com}
NAMESPACE=bodysense
CHANNEL_TAG=staging-latest
COMPOSE_PROJECT=bodysense-staging
STAGING_BIND_HOST=127.0.0.1
STAGING_WEB_PORT=20150
PROD_NETWORK=${STAGING_PROD_NETWORK:-docker_bodysense-network}
# MemAvailable + SwapFree, in MiB.
MIN_HEADROOM_DEPLOY_MB=${MIN_HEADROOM_DEPLOY_MB:-600}
MIN_HEADROOM_RUN_MB=${MIN_HEADROOM_RUN_MB:-300}

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

headroom_mb() {
  awk '/^MemAvailable:/{a=$2} /^SwapFree:/{s=$2} END{printf "%d", (a+s)/1024}' /proc/meminfo
}

web_ref="$REGISTRY/$NAMESPACE/bodysense-web:$CHANNEL_TAG"
api_ref="$REGISTRY/$NAMESPACE/bodysense-api:$CHANNEL_TAG"
ai_ref="$REGISTRY/$NAMESPACE/bodysense-ai-service:$CHANNEL_TAG"
runtime_ref="$REGISTRY/$NAMESPACE/bodysense-runtime:$CHANNEL_TAG"

compose() {
  STAGING_WEB_IMAGE="$web_ref" STAGING_API_IMAGE="$api_ref" STAGING_AI_IMAGE="$ai_ref" \
  STAGING_BIND_HOST="$STAGING_BIND_HOST" STAGING_WEB_PORT="$STAGING_WEB_PORT" \
  docker compose -p "$COMPOSE_PROJECT" \
    -f "$RUNTIME_ROOT/docker/docker-compose.staging.yml" -f "$OVERRIDE" \
    --env-file "$SECRET_ENV" "$@"
}

stop_staging() {
  log "stopping staging stack: $*"
  compose stop >/dev/null 2>&1 || docker ps -q --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" | xargs -r docker stop >/dev/null
}

guard_running() {
  local h; h=$(headroom_mb)
  if (( h < MIN_HEADROOM_RUN_MB )); then
    stop_staging "memory headroom ${h}MB < ${MIN_HEADROOM_RUN_MB}MB"
    fail "memory headroom too low (${h}MB); staging stopped to protect production"
  fi
}

image_revision() {
  docker image inspect "$1" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

container_revision() {
  local id image_id
  [[ -s "$RUNTIME_ROOT/docker/docker-compose.staging.yml" ]] || return 0
  id=$(compose ps -q "$1" 2>/dev/null || true)
  [[ -n "$id" ]] || return 0
  image_id=$(docker inspect "$id" --format '{{.Image}}' 2>/dev/null || true)
  [[ -n "$image_id" ]] || return 0
  docker image inspect "$image_id" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true
}

wait_healthy() {
  local service="$1" timeout="${2:-120}" start id status
  start=$(date +%s)
  while :; do
    guard_running
    id=$(compose ps -q "$service" 2>/dev/null || true)
    if [[ -n "$id" ]]; then
      status=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
      [[ "$status" == healthy ]] && return 0
    fi
    if (( $(date +%s) - start >= timeout )); then
      log "$service failed staging health wait"
      [[ -z "${id:-}" ]] || docker logs --tail 60 "$id" 2>&1 || true
      return 1
    fi
    sleep 3
  done
}

attach_prod_llm_network() {
  local id name
  id=$(compose ps -q ai-service 2>/dev/null || true)
  [[ -n "$id" ]] || return 0
  name=$(docker inspect "$id" --format '{{.Name}}'); name=${name#/}
  if ! docker inspect "$id" --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}' | grep -qw "$PROD_NETWORK"; then
    # No --alias: only the unique container name becomes resolvable on prod's network.
    docker network connect "$PROD_NETWORK" "$id"
    log "attached $name to $PROD_NETWORK (container-name DNS only)"
  fi
}

[[ -s "$SECRET_ENV" ]] || fail "missing staging secret env: $SECRET_ENV"
[[ -s "$OVERRIDE" ]] || fail "missing slim override: $OVERRIDE"
mkdir -p "$STATE_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another staging deploy check is already running'; exit 0; }

# Running-stack guard first: protects prod even when there is nothing new to deploy.
if [[ -s "$RUNTIME_ROOT/docker/docker-compose.staging.yml" ]] && [[ -n "$(compose ps -q 2>/dev/null)" ]]; then
  guard_running
  attach_prod_llm_network || true
fi

log 'checking ACR staging-latest pointers'
for ref in "$web_ref" "$api_ref" "$ai_ref" "$runtime_ref"; do
  docker pull -q "$ref" >/dev/null || fail "pull failed: $ref"
done
web_revision=$(image_revision "$web_ref"); api_revision=$(image_revision "$api_ref")
ai_revision=$(image_revision "$ai_ref"); runtime_revision=$(image_revision "$runtime_ref")
for pair in "web:$web_revision" "api:$api_revision" "ai:$ai_revision" "runtime:$runtime_revision"; do
  [[ -n "${pair#*:}" ]] || fail "${pair%%:*} staging image has no org.opencontainers.image.revision"
done
if ! [[ "$web_revision" == "$api_revision" && "$web_revision" == "$ai_revision" && "$web_revision" == "$runtime_revision" ]]; then
  log "staging channel not coherent yet: web=$web_revision api=$api_revision ai=$ai_revision runtime=$runtime_revision"
  exit 0
fi
desired_revision="$web_revision"
managed_revision=$(sed -n 's/^revision=//p' "$STATE_FILE" 2>/dev/null | tail -1 || true)

if $CHECK_ONLY; then
  log "STAGING_CANDIDATE=COHERENT revision=$desired_revision managed=${managed_revision:-none} headroom=$(headroom_mb)MB"
  exit 0
fi

if ! $FORCE && [[ "$managed_revision" == "$desired_revision" ]]; then
  all=true
  for s in web api ai-service; do [[ "$(container_revision "$s")" == "$desired_revision" ]] || all=false; done
  # restart: "no" means a stopped data service (reboot, guard stop, OOM) is redeployed here.
  for s in postgres redis; do [[ -n "$(compose ps -q "$s" 2>/dev/null)" ]] || all=false; done
  if $all; then log "already deployed staging revision $desired_revision"; exit 0; fi
  log "staging revision $desired_revision is not fully running; guarded (re)start"
fi

h=$(headroom_mb)
(( h >= MIN_HEADROOM_DEPLOY_MB )) || fail "refusing to deploy: memory headroom ${h}MB < ${MIN_HEADROOM_DEPLOY_MB}MB"

stage=$(mktemp -d "$STATE_DIR/staging-runtime-next.XXXXXX")
runtime_container=''
cleanup() { [[ -z "$runtime_container" ]] || docker rm -f "$runtime_container" >/dev/null 2>&1 || true; rm -rf "$stage"; }
trap cleanup EXIT
runtime_container=$(docker create "$runtime_ref" /bin/true)
docker cp "$runtime_container:/runtime/staging/." "$stage/"
docker rm "$runtime_container" >/dev/null; runtime_container=''
[[ -s "$stage/docker/docker-compose.staging.yml" ]] || fail 'candidate runtime missing docker/docker-compose.staging.yml'

STAGING_WEB_IMAGE="$web_ref" STAGING_API_IMAGE="$api_ref" STAGING_AI_IMAGE="$ai_ref" \
  docker compose -p "$COMPOSE_PROJECT" -f "$stage/docker/docker-compose.staging.yml" -f "$OVERRIDE" \
  --env-file "$SECRET_ENV" config -q || fail 'candidate compose + slim override failed validation'

rm -rf "$RUNTIME_ROOT.next" "$RUNTIME_ROOT.prev"
mkdir -p "$RUNTIME_ROOT.next"; cp -a "$stage/." "$RUNTIME_ROOT.next/"
[[ -d "$RUNTIME_ROOT" ]] && mv "$RUNTIME_ROOT" "$RUNTIME_ROOT.prev"
mv "$RUNTIME_ROOT.next" "$RUNTIME_ROOT"

log "deploying coherent slim staging revision $desired_revision (headroom ${h}MB)"
compose up -d --no-deps --no-build postgres redis
wait_healthy postgres 180 || fail 'staging postgres is unhealthy'
wait_healthy redis 90 || fail 'staging redis is unhealthy'
compose stop web >/dev/null 2>&1 || true
compose up -d --no-deps --no-build api
wait_healthy api 240 || fail 'staging API is unhealthy'
compose up -d --no-deps --no-build ai-service
attach_prod_llm_network
wait_healthy ai-service 240 || fail 'staging AI is unhealthy'
compose up -d --no-deps --no-build web
wait_healthy web 120 || fail 'staging Web is unhealthy'
for s in api ai-service web; do
  [[ "$(container_revision "$s")" == "$desired_revision" ]] || fail "staging $s revision mismatch"
done
curl -fsS --max-time 15 "http://${STAGING_BIND_HOST}:${STAGING_WEB_PORT}/api/health" >/dev/null \
  || fail 'staging ingress API health check failed'
guard_running

state_tmp="$STATE_FILE.tmp.$$"
printf 'revision=%s\nchannel=%s\nruntime_revision=%s\ndeployed_at=%s\n' \
  "$desired_revision" "$CHANNEL_TAG" "$runtime_revision" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$state_tmp"
mv -f "$state_tmp" "$STATE_FILE"
rm -rf "$RUNTIME_ROOT.prev"
log "STAGING_DEPLOY=PASS revision=$desired_revision headroom=$(headroom_mb)MB"
