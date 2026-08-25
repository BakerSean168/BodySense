#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

RUNTIME_DIR=${BODYSENSE_DEV_RUNTIME_DIR:-$ROOT/.runtime/dev}
ENV_FILE=${BODYSENSE_DEV_ENV_FILE:-$ROOT/.env.dev.local}
COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-bodysense-dev-infra}
export COMPOSE_PROJECT_NAME

# BodySense project block: 20100-20199.
export DEV_BIND_HOST=${DEV_BIND_HOST:-127.0.0.1}
export WEB_PORT=${WEB_PORT:-20100}
export API_PORT=${API_PORT:-20101}
export AI_SERVICE_PORT=${AI_SERVICE_PORT:-20102}
export DB_PORT=${DB_PORT:-20110}
export REDIS_PORT=${REDIS_PORT:-20111}
export LITELLM_PORT=${LITELLM_PORT:-20112}

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

export DB_USER=${DB_USER:-bodysense}
export DB_PASSWORD=${DB_PASSWORD:-bodysense123}
export DB_NAME=${DB_NAME:-bodysense}
export REDIS_PASSWORD=${REDIS_PASSWORD:-bodysense123}
export JWT_SECRET_KEY=${JWT_SECRET_KEY:-bodysense-dev-only-secret}
export LITELLM_MASTER_KEY=${LITELLM_MASTER_KEY:-sk-bodysense-dev-gateway}
export EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER:-hashing}
export CORS_ORIGINS=${CORS_ORIGINS:-http://localhost:${WEB_PORT},http://127.0.0.1:${WEB_PORT},https://gcp-dev-01.taile92a8e.ts.net:${WEB_PORT}}

compose=(docker compose -f docker/docker-compose.yml --profile dev)

mkdir -p "$RUNTIME_DIR"

pid_file() { printf '%s/%s.pid\n' "$RUNTIME_DIR" "$1"; }
log_file() { printf '%s/%s.log\n' "$RUNTIME_DIR" "$1"; }

is_running() {
  local file pid
  file=$(pid_file "$1")
  [[ -f "$file" ]] || return 1
  pid=$(cat "$file")
  kill -0 "$pid" 2>/dev/null || return 1
  local stat
  stat=$(ps -o stat= -p "$pid" 2>/dev/null | tr -d ' ' || true)
  [[ -n "$stat" && "$stat" != Z* ]]
}

start_process() {
  local name=$1 command=$2
  if is_running "$name"; then
    echo "$name already running pid=$(cat "$(pid_file "$name")")"
    return
  fi
  rm -f "$(pid_file "$name")"
  setsid bash -lc "$command" >>"$(log_file "$name")" 2>&1 < /dev/null &
  local pid=$!
  printf '%s\n' "$pid" >"$(pid_file "$name")"
  echo "$name started pid=$pid"
}

stop_process() {
  local name=$1 file pid
  file=$(pid_file "$name")
  [[ -f "$file" ]] || return 0
  pid=$(cat "$file")
  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.25
    done
    kill -KILL -- "-$pid" 2>/dev/null || true
  fi
  rm -f "$file"
  echo "$name stopped"
}

wait_http() {
  local name=$1 url=$2
  for _ in $(seq 1 80); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "$name health=PASS url=$url"
      return 0
    fi
    sleep 1
  done
  echo "$name health=FAIL url=$url" >&2
  tail -80 "$(log_file "$name")" 2>/dev/null || true
  return 1
}

up() {
  "${compose[@]}" up -d --wait --wait-timeout 120 postgres-dev redis-dev litellm-gateway

  # The Go API owns schema migration/extension initialization. Start it before
  # the Python knowledge pool so a brand-new dev volume is bootstrapped deterministically.
  start_process api "cd '$ROOT/apps/api' && exec env API_HOST=127.0.0.1 API_PORT='${API_PORT}' DB_HOST=127.0.0.1 DB_PORT='${DB_PORT}' DB_NAME='${DB_NAME}' DB_USER='${DB_USER}' DB_PASSWORD='${DB_PASSWORD}' DB_SSLMODE=disable REDIS_HOST=127.0.0.1 REDIS_PORT='${REDIS_PORT}' REDIS_PASSWORD='${REDIS_PASSWORD}' JWT_SECRET_KEY='${JWT_SECRET_KEY}' CORS_ORIGINS='${CORS_ORIGINS}' AI_SERVICE_URL='http://127.0.0.1:${AI_SERVICE_PORT}' go run ./cmd/server"
  wait_http api "http://127.0.0.1:${API_PORT}/api/health"

  start_process ai "cd '$ROOT/apps/ai-service' && exec env DATABASE_URL='postgresql://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${DB_NAME}' LITELLM_BASE_URL='http://127.0.0.1:${LITELLM_PORT}/v1' LITELLM_API_KEY='${LITELLM_MASTER_KEY}' EMBEDDING_PROVIDER='${EMBEDDING_PROVIDER}' CORS_ORIGINS='${CORS_ORIGINS}' uv run --extra ocr uvicorn src.main:app --host 127.0.0.1 --port '${AI_SERVICE_PORT}' --reload"
  wait_http ai "http://127.0.0.1:${AI_SERVICE_PORT}/health"

  start_process web "cd '$ROOT' && exec env BODYSENSE_WEB_PORT='${WEB_PORT}' VITE_DEV_API_TARGET='http://127.0.0.1:${API_PORT}' pnpm exec vite --config apps/web/vite.config.ts --host 127.0.0.1 --port '${WEB_PORT}' --strictPort"
  wait_http web "http://127.0.0.1:${WEB_PORT}"
}

down() {
  stop_process web
  stop_process api
  stop_process ai
  "${compose[@]}" down --remove-orphans
}

status() {
  echo "BodySense dev ports: web=${WEB_PORT} api=${API_PORT} ai=${AI_SERVICE_PORT} postgres=${DB_PORT} redis=${REDIS_PORT} litellm=${LITELLM_PORT}"
  for name in web api ai; do
    if is_running "$name"; then
      echo "$name=RUNNING pid=$(cat "$(pid_file "$name")")"
    else
      echo "$name=STOPPED"
    fi
  done
  "${compose[@]}" ps postgres-dev redis-dev litellm-gateway || true
  curl -fsS "http://127.0.0.1:${API_PORT}/api/health" || true
  echo
}

case "${1:-status}" in
  up) up ;;
  down) down ;;
  restart) down; up ;;
  status) status ;;
  logs) tail -n "${2:-120}" "$RUNTIME_DIR"/*.log 2>/dev/null || true ;;
  *) echo "usage: $0 {up|down|restart|status|logs [lines]}" >&2; exit 2 ;;
esac
