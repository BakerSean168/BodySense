#!/usr/bin/env bash
# Persistent remote direct-dev, managed by standard Process Compose inside tmux.
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"
SESSION=${BODYSENSE_DEV_TMUX_SESSION:-bodysense-dev}
PORT=20120
PC=${BODYSENSE_PROCESS_COMPOSE_BIN:-$HOME/.local/bin/process-compose}
SERVICES=(document api ai web)

require_main() {
  if [[ $(git -C "$ROOT" branch --show-current) != main ]]; then
    echo "The persistent $SESSION session must use the main worktree: $ROOT" >&2
    exit 1
  fi
}

require_service() {
  local item
  for item in "${SERVICES[@]}"; do
    [[ "$item" == "${1:-}" ]] && return
  done
  echo "Unknown service: ${1:-}; expected ${SERVICES[*]}" >&2
  exit 2
}

pc() { "$PC" -p "$PORT" "$@"; }

has_session() { tmux has-session -t "=$SESSION" 2>/dev/null; }

up() {
  require_main
  command -v tmux >/dev/null
  test -x "$PC" || { echo "Install Process Compose v1.124.0 to $PC" >&2; exit 1; }
  "$ROOT/scripts/dev-infra.sh" up
  if has_session; then
    if pc process list >/dev/null 2>&1; then
      echo "Session $SESSION is already running."
      status
      return
    fi
    echo "Session $SESSION exists but Process Compose is not responding; inspect before overwriting." >&2
    exit 1
  fi
  cd "$ROOT"
  tmux new-session -d -s "$SESSION" -n services -c "$ROOT" \
    "exec '$PC' -f '$ROOT/process-compose.yaml' --disable-dotenv --address 127.0.0.1 -p '$PORT'"
  tmux new-window -d -t "$SESSION:" -n ops -c "$ROOT"
  tmux select-window -t "$SESSION:services"
  echo "Started: tmux attach -t $SESSION"
  echo "Services can be restarted independently in Process Compose TUI or via: bash scripts/dev-tmux.sh restart api"
  echo "Preview: ssh -N -L 20100:127.0.0.1:20100 oracle2"
}

status() {
  if has_session; then
    tmux list-windows -t "$SESSION" -F '  tmux: #{window_name} dead=#{pane_dead}'
  else
    echo "tmux session $SESSION: not running"
  fi
  if test -x "$PC"; then
    pc process list || true
  fi
  "$ROOT/scripts/dev-infra.sh" status
}

reset_db() {
  require_main
  if [[ ${COMPOSE_PROJECT_NAME:-bodysense-dev-infra} != bodysense-dev-infra ||
        "$DB_NAME" != bodysense || "$DB_PORT" != 20110 ||
        "$DB_HOST" != 127.0.0.1 || "$DEV_BIND_HOST" != 127.0.0.1 ]]; then
    echo "Refusing dev database reset: expected canonical BodySense dev settings" >&2
    exit 1
  fi
  local app_port
  for app_port in 20100 20101 20102 20103; do
    if ss -H -ltn | grep -qE ":${app_port}[[:space:]]"; then
      echo "Refusing dev database reset: application port $app_port is still active" >&2
      exit 1
    fi
  done
  local container label published active
  container=bodysense-dev-infra-postgres-dev-1
  label=$(docker inspect "$container" --format '{{index .Config.Labels "com.docker.compose.project"}}')
  published=$(docker port "$container" 5432/tcp)
  if [[ "$label" != bodysense-dev-infra || "$published" != 127.0.0.1:20110 ]]; then
    echo "Refusing dev database reset: container identity or port mismatch" >&2
    exit 1
  fi
  active=$(docker exec "$container" psql -U "$DB_USER" -d postgres -Atc \
    "SELECT count(*) FROM pg_stat_activity WHERE datname='bodysense' AND pid<>pg_backend_pid()")
  if [[ "$active" != 0 ]]; then
    echo "Refusing dev database reset: $active connection(s) are active" >&2
    exit 1
  fi
  docker exec "$container" psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS "bodysense" WITH (FORCE);'
  docker exec "$container" psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c 'CREATE DATABASE "bodysense";'
  echo "Created an empty development database; Go API will initialize the baseline on next start."
}

case "${1:-status}" in
  up) up ;;
  attach) exec tmux attach -t "$SESSION" ;;
  status) status ;;
  start|stop|restart|logs)
    action=$1
    require_service "${2:-}"
    case "$action" in
      start|restart) require_main ;;
    esac
    if [[ "$action" == logs ]]; then pc process logs "$2"; else pc process "$action" "$2"; fi
    ;;
  down)
    if test -x "$PC"; then pc down || true; fi
    if has_session; then tmux kill-session -t "$SESSION"; fi
    echo "Application processes stopped. Docker infra remains running."
    ;;
  reset-db) reset_db ;;
  *)
    echo "usage: $0 {up|attach|status|start|stop|restart|logs SERVICE|down|reset-db}" >&2
    exit 2
    ;;
esac
