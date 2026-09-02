#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"

mode=${1:-all}
pids=()
names=()
cleaned_up=0

port_is_busy() {
  local port=$1
  ss -H -ltnp 2>/dev/null | grep -Eq "127\\.0\\.0\\.1:${port}\\b"
}

assert_port_free() {
  local name=$1 port=$2
  if port_is_busy "$port"; then
    echo "$name direct-dev port $port is already in use:" >&2
    ss -H -ltnp 2>/dev/null | grep -E "127\\.0\\.0\\.1:${port}\\b" >&2 || true
    echo "Stop the previous dev process before starting another one." >&2
    exit 1
  fi
}

start_service() {
  local name=$1 cwd=$2
  shift 2
  echo "[$name] starting"
  # The single-quoted child script intentionally expands $1/$@ inside that child shell.
  # shellcheck disable=SC2016
  setsid bash -c 'cd "$1"; shift; exec "$@"' _ "$cwd" "$@" &
  local pid=$!
  pids+=("$pid")
  names+=("$name")
  echo "[$name] supervisor_pid=$pid"
}

wait_http() {
  local name=$1 url=$2 pid=$3
  for _ in $(seq 1 90); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "[$name] health=PASS url=$url"
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "[$name] exited before becoming healthy" >&2
      wait "$pid" || true
      return 1
    fi
    sleep 1
  done
  echo "[$name] health=FAIL url=$url" >&2
  return 1
}

cleanup() {
  [[ "$cleaned_up" = 0 ]] || return 0
  cleaned_up=1
  trap - INT TERM EXIT

  local pid
  for pid in "${pids[@]:-}"; do
    [[ -n "$pid" ]] || continue
    kill -INT -- "-$pid" 2>/dev/null || true
  done

  for _ in $(seq 1 40); do
    local any=0
    for pid in "${pids[@]:-}"; do
      [[ -n "$pid" ]] || continue
      if kill -0 "$pid" 2>/dev/null; then
        any=1
        break
      fi
    done
    [[ "$any" = 0 ]] && break
    sleep 0.25
  done

  for pid in "${pids[@]:-}"; do
    [[ -n "$pid" ]] || continue
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM -- "-$pid" 2>/dev/null || true
    fi
  done
  sleep 0.5
  for pid in "${pids[@]:-}"; do
    [[ -n "$pid" ]] || continue
    kill -KILL -- "-$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
}

# Invoked indirectly by the INT/TERM traps below.
# shellcheck disable=SC2317
on_signal() {
  cleanup
  echo "BodySense direct dev stopped; persistent infrastructure is still running."
  exit 0
}

trap on_signal INT TERM
trap cleanup EXIT

start_document() {
  assert_port_free document "$DOCUMENT_SERVICE_PORT"
  "$ROOT/scripts/dev-infra.sh" up
  (cd "$ROOT/apps/ai-service" && uv run --extra document-ocr python scripts/ensure_health_document_models.py)
  start_service document "$ROOT/apps/ai-service" uv run --extra document-ocr uvicorn src.document_main:app --host 127.0.0.1 --port "$DOCUMENT_SERVICE_PORT"
  wait_http document "http://127.0.0.1:${DOCUMENT_SERVICE_PORT}/health" "${pids[-1]}"
}

start_api() {
  assert_port_free api "$API_PORT"
  start_document
  start_service api "$ROOT/apps/api" go run ./cmd/server
  wait_http api "http://127.0.0.1:${API_PORT}/api/health" "${pids[-1]}"
}

start_ai() {
  assert_port_free ai "$AI_SERVICE_PORT"
  "$ROOT/scripts/dev-infra.sh" up
  (cd "$ROOT/apps/ai-service" && uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_pose_model.py && uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_health_document_models.py)
  start_service ai "$ROOT/apps/ai-service" uv run --extra ocr --extra pose --extra document-ocr uvicorn src.main:app --reload --host 127.0.0.1 --port "$AI_SERVICE_PORT"
  wait_http ai "http://127.0.0.1:${AI_SERVICE_PORT}/health" "${pids[-1]}"
}

start_web() {
  assert_port_free web "$WEB_PORT"
  start_service web "$ROOT" pnpm exec vite --config apps/web/vite.config.ts --host 127.0.0.1 --port "$WEB_PORT" --strictPort
  wait_http web "http://127.0.0.1:${WEB_PORT}/" "${pids[-1]}"
}

case "$mode" in
  all)
    assert_port_free api "$API_PORT"
    assert_port_free ai "$AI_SERVICE_PORT"
    assert_port_free document "$DOCUMENT_SERVICE_PORT"
    assert_port_free web "$WEB_PORT"
    "$ROOT/scripts/dev-infra.sh" up

    (cd "$ROOT/apps/ai-service" && uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_pose_model.py && uv run --extra document-ocr python scripts/ensure_health_document_models.py)
    start_service document "$ROOT/apps/ai-service" uv run --extra document-ocr uvicorn src.document_main:app --host 127.0.0.1 --port "$DOCUMENT_SERVICE_PORT"
    wait_http document "http://127.0.0.1:${DOCUMENT_SERVICE_PORT}/health" "${pids[-1]}"

    start_service api "$ROOT/apps/api" go run ./cmd/server
    wait_http api "http://127.0.0.1:${API_PORT}/api/health" "${pids[-1]}"

    start_service ai "$ROOT/apps/ai-service" uv run --extra ocr --extra pose --extra document-ocr uvicorn src.main:app --reload --host 127.0.0.1 --port "$AI_SERVICE_PORT"
    wait_http ai "http://127.0.0.1:${AI_SERVICE_PORT}/health" "${pids[-1]}"

    start_service web "$ROOT" pnpm exec vite --config apps/web/vite.config.ts --host 127.0.0.1 --port "$WEB_PORT" --strictPort
    wait_http web "http://127.0.0.1:${WEB_PORT}/" "${pids[-1]}"

    echo "BodySense direct dev is ready:"
    echo "  web: http://127.0.0.1:${WEB_PORT}"
    echo "  api: http://127.0.0.1:${API_PORT}"
    echo "  ai:  http://127.0.0.1:${AI_SERVICE_PORT}"
    echo "  document: http://127.0.0.1:${DOCUMENT_SERVICE_PORT}"
    if [[ -n "${BODYSENSE_DEV_PUBLIC_URL:-}" ]]; then
      echo "  remote: ${BODYSENSE_DEV_PUBLIC_URL}"
    fi
    ;;
  api) start_api ;;
  ai) start_ai ;;
  document) start_document ;;
  web) start_web ;;
  *)
    echo "usage: $0 {all|web|api|ai|document}" >&2
    exit 2
    ;;
esac

set +e
wait -n "${pids[@]}"
rc=$?
set -e
if [[ "$rc" -ne 0 ]]; then
  echo "A direct-dev application process exited with status $rc; stopping the remaining application processes." >&2
fi
cleanup
exit "$rc"
