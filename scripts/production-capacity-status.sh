#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=install-production-capacity.sh
source "$SCRIPT_DIR/install-production-capacity.sh"

COMMAND="${1:-status}"
ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_FILE="$ROOT/.capacity-state"

read_public_env() {
  local key="$1" default="${2:-}" value=""
  if [ -s "$PUBLIC_ENV" ]; then
    value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  fi
  printf '%s' "${value:-$default}"
}

require_uint() {
  local name="$1" value="$2"
  [[ "$value" =~ ^[0-9]+$ ]] || { echo "invalid ${name}=${value}" >&2; return 2; }
}

load_thresholds() {
  MEM_WARN="${CAPACITY_MEMORY_WARN_AVAILABLE_PCT:-$(read_public_env CAPACITY_MEMORY_WARN_AVAILABLE_PCT 15)}"
  MEM_CRIT="${CAPACITY_MEMORY_CRITICAL_AVAILABLE_PCT:-$(read_public_env CAPACITY_MEMORY_CRITICAL_AVAILABLE_PCT 8)}"
  SWAP_WARN="${CAPACITY_SWAP_WARN_USED_PCT:-$(read_public_env CAPACITY_SWAP_WARN_USED_PCT 25)}"
  SWAP_CRIT="${CAPACITY_SWAP_CRITICAL_USED_PCT:-$(read_public_env CAPACITY_SWAP_CRITICAL_USED_PCT 60)}"
  DISK_WARN="${CAPACITY_DISK_WARN_USED_PCT:-$(read_public_env CAPACITY_DISK_WARN_USED_PCT 80)}"
  DISK_CRIT="${CAPACITY_DISK_CRITICAL_USED_PCT:-$(read_public_env CAPACITY_DISK_CRITICAL_USED_PCT 90)}"
  CONTAINER_WARN="${CAPACITY_CONTAINER_MEMORY_WARN_PCT:-$(read_public_env CAPACITY_CONTAINER_MEMORY_WARN_PCT 80)}"
  CONTAINER_CRIT="${CAPACITY_CONTAINER_MEMORY_CRITICAL_PCT:-$(read_public_env CAPACITY_CONTAINER_MEMORY_CRITICAL_PCT 92)}"
  MIN_SWAP_GIB="${CAPACITY_SWAP_GIB:-$(read_public_env CAPACITY_SWAP_GIB 2)}"
  local name value
  for name in MEM_WARN MEM_CRIT SWAP_WARN SWAP_CRIT DISK_WARN DISK_CRIT CONTAINER_WARN CONTAINER_CRIT MIN_SWAP_GIB; do
    value="${!name}"
    require_uint "$name" "$value" || return 2
  done
  (( MEM_CRIT < MEM_WARN && MEM_WARN <= 100 )) || { echo 'invalid memory capacity threshold order' >&2; return 2; }
  (( SWAP_WARN < SWAP_CRIT && SWAP_CRIT <= 100 )) || { echo 'invalid swap capacity threshold order' >&2; return 2; }
  (( DISK_WARN < DISK_CRIT && DISK_CRIT <= 100 )) || { echo 'invalid disk capacity threshold order' >&2; return 2; }
  (( CONTAINER_WARN < CONTAINER_CRIT && CONTAINER_CRIT <= 100 )) || { echo 'invalid container capacity threshold order' >&2; return 2; }
  (( MIN_SWAP_GIB >= 1 && MIN_SWAP_GIB <= 4 )) || { echo 'invalid minimum swap GiB' >&2; return 2; }
}

numeric_or_zero() {
  local value="${1:-}"
  [[ "$value" =~ ^[0-9]+$ ]] && printf '%s' "$value" || printf '0'
}

STATUS=PASS
ISSUES=()
warn() { [ "$STATUS" = CRITICAL ] || STATUS=WARN; ISSUES+=("$1"); }
critical() { STATUS=CRITICAL; ISSUES+=("$1"); }

join_issues() {
  local IFS=';'
  printf '%s' "${ISSUES[*]:-none}"
}

# Pure policy evaluation used by both live status collection and deterministic tests.
evaluate_metrics() {
  local mem_available_pct="$1" swap_total_gib="$2" swap_used_pct="$3" disk_used_pct="$4"
  local container_mem_pct="$5" restarts="$6" oom="$7" stale_runs="$8" unbounded="$9"
  local bad_containers="${10}" deploy_timer="${11}" deploy_result="${12}" dr_status="${13}"
  STATUS=PASS
  ISSUES=()
  (( mem_available_pct <= MEM_CRIT )) && critical "memory_available_${mem_available_pct}pct" || {
    (( mem_available_pct <= MEM_WARN )) && warn "memory_available_${mem_available_pct}pct" || true
  }
  (( swap_total_gib < MIN_SWAP_GIB )) && warn "swap_below_${MIN_SWAP_GIB}gib" || true
  (( swap_used_pct >= SWAP_CRIT )) && critical "swap_used_${swap_used_pct}pct" || {
    (( swap_used_pct >= SWAP_WARN )) && warn "swap_used_${swap_used_pct}pct" || true
  }
  (( disk_used_pct >= DISK_CRIT )) && critical "disk_used_${disk_used_pct}pct" || {
    (( disk_used_pct >= DISK_WARN )) && warn "disk_used_${disk_used_pct}pct" || true
  }
  (( container_mem_pct >= CONTAINER_CRIT )) && critical "container_memory_${container_mem_pct}pct" || {
    (( container_mem_pct >= CONTAINER_WARN )) && warn "container_memory_${container_mem_pct}pct" || true
  }
  (( oom > 0 )) && critical "container_oom_${oom}" || true
  (( bad_containers > 0 )) && critical "containers_unhealthy_${bad_containers}" || true
  (( restarts > 0 )) && warn "container_restarts_${restarts}" || true
  (( stale_runs > 0 )) && warn "stale_runs_${stale_runs}" || true
  (( unbounded > 0 )) && warn "unbounded_containers_${unbounded}" || true
  [ "$deploy_timer" = active ] || critical "deploy_timer_${deploy_timer}"
  [ "$deploy_result" = success ] || warn "deploy_service_result_${deploy_result}"
  [ "$dr_status" != fail ] || critical "dr_status_failed"

  printf 'CAPACITY_STATUS=%s\n' "$STATUS"
  printf 'ISSUES=%s\n' "$(join_issues)"
  printf 'MEM_AVAILABLE_PCT=%s\n' "$mem_available_pct"
  printf 'SWAP_TOTAL_GIB=%s\n' "$swap_total_gib"
  printf 'SWAP_USED_PCT=%s\n' "$swap_used_pct"
  printf 'ROOT_DISK_USED_PCT=%s\n' "$disk_used_pct"
  printf 'CONTAINER_MAX_MEMORY_PCT=%s\n' "$container_mem_pct"
  printf 'CONTAINER_RESTARTS=%s\n' "$restarts"
  printf 'CONTAINER_OOM=%s\n' "$oom"
  printf 'STALE_RUNS=%s\n' "$stale_runs"
  printf 'UNBOUNDED_CONTAINERS=%s\n' "$unbounded"
  printf 'BAD_CONTAINERS=%s\n' "$bad_containers"
  printf 'DEPLOY_TIMER=%s\n' "$deploy_timer"
  printf 'DEPLOY_RESULT=%s\n' "$deploy_result"
  printf 'DR_STATUS=%s\n' "$dr_status"

  case "$STATUS" in PASS) return 0 ;; WARN) return 1 ;; *) return 2 ;; esac
}

if [ "$COMMAND" = evaluate ]; then
  load_thresholds
  evaluate_metrics \
    "${CAPACITY_TEST_MEM_AVAILABLE_PCT:?}" "${CAPACITY_TEST_SWAP_TOTAL_GIB:?}" \
    "${CAPACITY_TEST_SWAP_USED_PCT:?}" "${CAPACITY_TEST_DISK_USED_PCT:?}" \
    "${CAPACITY_TEST_CONTAINER_MEMORY_PCT:?}" "${CAPACITY_TEST_RESTARTS:-0}" \
    "${CAPACITY_TEST_OOM:-0}" "${CAPACITY_TEST_STALE_RUNS:-0}" \
    "${CAPACITY_TEST_UNBOUNDED:-0}" "${CAPACITY_TEST_BAD_CONTAINERS:-0}" \
    "${CAPACITY_TEST_DEPLOY_TIMER:-active}" "${CAPACITY_TEST_DEPLOY_RESULT:-success}" \
    "${CAPACITY_TEST_DR_STATUS:-disabled}"
  exit $?
fi

[ -s "$PUBLIC_ENV" ] || { echo "missing $PUBLIC_ENV" >&2; exit 2; }
[ -s "$SECRET_ENV" ] || { echo "missing $SECRET_ENV" >&2; exit 2; }
[ -s "$COMPOSE" ] || { echo "missing $COMPOSE" >&2; exit 2; }
load_thresholds

compose() {
  docker compose -p "${BODYSENSE_COMPOSE_PROJECT:-docker}" -f "$COMPOSE" --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" "$@"
}

if [ "$COMMAND" = cleanup ]; then
  exec 9>"$ROOT/.deploy.lock"
  flock -n 9 || { echo 'capacity cleanup deferred: deploy transaction is active'; exit 0; }
  retention="$(read_public_env CAPACITY_RETENTION_DAYS 14)"
  prune_until="$(read_public_env CAPACITY_DOCKER_PRUNE_UNTIL 168h)"
  find "$ROOT/backups" -type f -name 'bodysense-pre-*.dump*' -mtime "+$retention" -delete 2>/dev/null || true
  find "$ROOT/runtime-backups" -mindepth 1 -maxdepth 1 -type d -mtime "+$retention" -exec rm -rf {} + 2>/dev/null || true
  docker container prune -f --filter "until=$prune_until" >/dev/null
  # Intentionally omit -a: only dangling images are eligible. Running images and
  # tagged rollback artifacts remain protected. Never prune volumes here.
  docker image prune -f --filter "until=$prune_until" >/dev/null
  docker builder prune -f --filter "until=$prune_until" >/dev/null
  echo CAPACITY_CLEANUP=PASS
  exit 0
fi

[ "$COMMAND" = status ] || { echo "usage: $0 {status|cleanup|evaluate}" >&2; exit 2; }

mem_total_kb=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)
mem_available_kb=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)
mem_available_pct=$(( mem_available_kb * 100 / mem_total_kb ))
swap_total_kb=$(awk '/^SwapTotal:/ {print $2}' /proc/meminfo)
swap_free_kb=$(awk '/^SwapFree:/ {print $2}' /proc/meminfo)
swap_total_gib=$(capacity_swap_effective_gib "$swap_total_kb" "$MIN_SWAP_GIB")
if (( swap_total_kb > 0 )); then swap_used_pct=$(( (swap_total_kb - swap_free_kb) * 100 / swap_total_kb )); else swap_used_pct=0; fi
disk_used_pct=$(df -P / | awk 'NR==2 {gsub(/%/,"",$5); print $5}')

restarts=0
oom=0
bad_containers=0
unbounded=0
container_max_pct=0
for service in postgres redis litellm-gateway ai-service api web caddy; do
  id=$(compose ps -q "$service" 2>/dev/null || true)
  if [ -z "$id" ]; then ((bad_containers+=1)); continue; fi
  state=$(docker inspect "$id" --format '{{.State.Status}}' 2>/dev/null || echo unknown)
  health=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' 2>/dev/null || echo unknown)
  [ "$state" = running ] || ((bad_containers+=1))
  case "$health" in healthy|none) ;; *) ((bad_containers+=1)) ;; esac
  r=$(numeric_or_zero "$(docker inspect "$id" --format '{{.RestartCount}}' 2>/dev/null || true)"); restarts=$((restarts + r))
  o=$(numeric_or_zero "$(docker inspect "$id" --format '{{if .State.OOMKilled}}1{{else}}0{{end}}' 2>/dev/null || true)"); oom=$((oom + o))
  limit=$(numeric_or_zero "$(docker inspect "$id" --format '{{.HostConfig.Memory}}' 2>/dev/null || true)"); (( limit > 0 )) || ((unbounded+=1))
  pct_raw=$(docker stats --no-stream --format '{{.MemPerc}}' "$id" 2>/dev/null | tr -d '%' | cut -d. -f1 || true)
  pct=$(numeric_or_zero "$pct_raw"); (( pct > container_max_pct )) && container_max_pct=$pct || true
done

deploy_timer=$(systemctl is-active bodysense-deploy-watch.timer 2>/dev/null || true); deploy_timer=${deploy_timer:-unknown}
deploy_result=$(systemctl show bodysense-deploy-watch.service -p Result --value 2>/dev/null || true); deploy_result=${deploy_result:-unknown}

stale_runs=0
lease_columns=$(compose exec -T postgres psql -U "$(read_public_env DB_USER bodysense)" -d "$(read_public_env DB_NAME bodysense)" -Atc \
  "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='runs' AND column_name IN ('lease_owner','lease_expires_at');" 2>/dev/null || echo unknown)
if [ "$lease_columns" = 2 ]; then
  stale_runs=$(compose exec -T postgres psql -U "$(read_public_env DB_USER bodysense)" -d "$(read_public_env DB_NAME bodysense)" -Atc \
    "SELECT count(*) FROM runs WHERE status='running' AND (lease_owner IS NULL OR btrim(lease_owner)='' OR lease_expires_at IS NULL OR lease_expires_at <= now());" 2>/dev/null || echo 0)
fi

dr_status=disabled
if grep -Eq '^DR_ENABLED=true$' "$PUBLIC_ENV" "$SECRET_ENV" 2>/dev/null; then
  if "$ROOT/scripts/production-postgres-dr.sh" status >/dev/null 2>&1; then dr_status=pass; else dr_status=fail; fi
fi

set +e
output=$(evaluate_metrics "$mem_available_pct" "$swap_total_gib" "$swap_used_pct" "$disk_used_pct" "$container_max_pct" "$restarts" "$oom" "$stale_runs" "$unbounded" "$bad_containers" "$deploy_timer" "$deploy_result" "$dr_status")
code=$?
set -e
tmp="$STATE_FILE.tmp.$$"
{
  printf 'checked_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf '%s\n' "$output"
} > "$tmp"
chmod 0644 "$tmp"
mv "$tmp" "$STATE_FILE"
printf '%s\n' "$output"
exit "$code"
