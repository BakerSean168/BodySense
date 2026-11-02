#!/usr/bin/env bash
# BodySense direct-dev: persistent infrastructure and independently controlled
# application processes, each in its own tmux window.
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"
SESSION=${BODYSENSE_DEV_TMUX_SESSION:-bodysense-dev}
SERVICES=(document api ai web)

require_main() {
  local branch
  branch=$(git -C "$ROOT" branch --show-current)
  if [[ "$branch" != main ]]; then
    echo "Refusing to manage the stable $SESSION session from branch '$branch'. Run the main-worktree copy of this script." >&2
    exit 1
  fi
}

has_session() { tmux has-session -t "=$SESSION" 2>/dev/null; }
has_window() {
  has_session && tmux list-windows -t "=$SESSION" -F '#{window_name}' | grep -Fxq -- "$1"
}
pane_dead() {
  [[ $(tmux list-panes -t "$SESSION:$1" -F '#{pane_dead}' | head -1) == 1 ]]
}
ensure_session() {
  if ! has_session; then
    tmux new-session -d -s "$SESSION" -n ops -c "$ROOT"
    echo "Created tmux session $SESSION (ops window is an interactive shell)."
  fi
}
is_service() {
  local s
  for s in "${SERVICES[@]}"; do [[ "$s" == "$1" ]] && return 0; done
  echo "Unknown service '$1'; choose: ${SERVICES[*]}" >&2
  return 1
}
url_for() {
  case "$1" in
    document) printf 'http://127.0.0.1:%s/health\n' "$DOCUMENT_SERVICE_PORT" ;;
    api) printf 'http://127.0.0.1:%s/api/health\n' "$API_PORT" ;;
    ai) printf 'http://127.0.0.1:%s/health\n' "$AI_SERVICE_PORT" ;;
    web) printf 'http://127.0.0.1:%s/\n' "$WEB_PORT" ;;
  esac
}
wait_healthy() {
  local service=$1 url i
  url=$(url_for "$service")
  for ((i=0; i<180; i++)); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      echo "[$service] HEALTHY $url"
      return 0
    fi
    if ! has_window "$service" || pane_dead "$service"; then
      echo "[$service] exited before becoming healthy; inspect: $0 logs $service" >&2
      return 1
    fi
    sleep 1
  done
  echo "[$service] timed out waiting for $url; inspect: $0 logs $service" >&2
  return 1
}
start_one() {
  local service=$1 cmd
  is_service "$service"
  ensure_session
  cmd="exec bash '$ROOT/scripts/dev-pane.sh' '$service'"
  if has_window "$service"; then
    if ! pane_dead "$service"; then
      echo "[$service] already running"
      return 0
    fi
    tmux respawn-window -t "$SESSION:$service" -c "$ROOT" "$cmd"
  else
    tmux new-window -d -t "$SESSION:" -n "$service" -c "$ROOT" "$cmd"
    tmux set-window-option -t "$SESSION:$service" remain-on-exit on
  fi
  echo "[$service] started in $SESSION:$service"
}
stop_one() {
  local service=$1 i
  is_service "$service"
  if ! has_window "$service"; then
    echo "[$service] already stopped"
    return 0
  fi
  if pane_dead "$service"; then
    echo "[$service] already exited (logs preserved in tmux)"
    return 0
  fi
  # Send SIGINT through the terminal so Go Air/Uvicorn/Vite can stop children.
  tmux send-keys -t "$SESSION:$service" C-c
  for ((i=0; i<30; i++)); do
    if ! has_window "$service" || pane_dead "$service"; then
      echo "[$service] stopped"
      return 0
    fi
    sleep 0.2
  done
  echo "[$service] did not exit after SIGINT; inspect manually before restarting" >&2
  return 1
}
up() {
  require_main
  "$ROOT/scripts/dev-infra.sh" up
  ensure_session
  local s
  for s in "${SERVICES[@]}"; do
    start_one "$s"
    wait_healthy "$s"
  done
  echo "Ready. Attach with: tmux attach -t $SESSION"
  echo "Windows: ops, document, api, ai, web"
  echo "Preview via SSH tunnel: ssh -N -L $WEB_PORT:127.0.0.1:$WEB_PORT oracle2"
}
status() {
  printf 'Session: %s\n' "$SESSION"
  if has_session; then
    tmux list-windows -t "$SESSION" -F '  #{window_name}: dead=#{pane_dead} command=#{pane_current_command}'
  else
    echo "  no session"
  fi
  local s url
  for s in "${SERVICES[@]}"; do
    url=$(url_for "$s")
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      echo "  $s: HEALTHY ($url)"
    else
      echo "  $s: UNAVAILABLE ($url)"
    fi
  done
  "$ROOT/scripts/dev-infra.sh" status
}
reset_db() {
  require_main
  # Extra safety: limit destructive SQL to the dedicated BodySense dev instance.
  if [[ ${COMPOSE_PROJECT_NAME:-bodysense-dev-infra} != bodysense-dev-infra ||
        "$DB_NAME" != bodysense || "$DB_PORT" != 20110 ||
        "$DB_HOST" != 127.0.0.1 || "$DEV_BIND_HOST" != 127.0.0.1 ]]; then
    echo "Refusing reset: noncanonical development database configuration" >&2
    exit 1
  fi
  local s container label published active
  for s in "${SERVICES[@]}"; do
    if has_window "$s" && ! pane_dead "$s"; then
      echo "Stop all development application windows before resetting the database ($s is running)." >&2
      exit 1
    fi
  done
  container=bodysense-dev-infra-postgres-dev-1
  label=$(docker inspect "$container" --format '{{index .Config.Labels "com.docker.compose.project"}}')
  published=$(docker port "$container" 5432/tcp)
  if [[ "$label" != bodysense-dev-infra || "$published" != "127.0.0.1:20110" ]]; then
    echo "Refusing reset: container identity/port mismatch" >&2
    exit 1
  fi
  active=$(docker exec "$container" psql -U "$DB_USER" -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname = 'bodysense' AND pid <> pg_backend_pid()")
  if [[ "$active" != 0 ]]; then
    echo "Refusing reset: $active client connection(s) still use the development database." >&2
    exit 1
  fi
  docker exec "$container" psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS "bodysense" WITH (FORCE);'
  docker exec "$container" psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c 'CREATE DATABASE "bodysense";'
  echo "Recreated empty BodySense development database. Go API will initialize the schema on startup; no existing rows are migrated."
}
usage() {
  echo "usage: $0 {up|attach|status|start SERVICE|stop SERVICE|restart SERVICE|logs SERVICE|down|reset-db}"
}
action=${1:-status}
case "$action" in
  up) up ;;
  attach) exec tmux attach -t "$SESSION" ;;
  status) status ;;
  start)
    require_main
    is_service "${2:-}"
    start_one "$2"
    wait_healthy "$2"
    ;;
  stop)
    is_service "${2:-}"
    stop_one "$2"
    ;;
  restart)
    require_main
    is_service "${2:-}"
    stop_one "$2"
    start_one "$2"
    wait_healthy "$2"
    ;;
  logs)
    is_service "${2:-}"
    if has_window "$2"; then
      tmux capture-pane -p -t "$SESSION:$2" -S -160
    else
      echo "No window for $2" >&2
      exit 1
    fi
    ;;
  down)
    if has_session; then tmux kill-session -t "$SESSION"; fi
    echo "Stopped application session; development Docker infrastructure retained."
    ;;
  reset-db) reset_db ;;
  *) usage >&2; exit 2 ;;
esac
