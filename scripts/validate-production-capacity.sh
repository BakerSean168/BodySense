#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

bash -n scripts/production-capacity-status.sh scripts/install-production-capacity.sh \
  scripts/setup-server.sh scripts/production-deploy-watch.sh

run_eval() {
  local expected_code="$1" expected_status="$2" expected_issue="$3"
  shift 3
  local output code
  set +e
  output=$(env "$@" scripts/production-capacity-status.sh evaluate 2>&1)
  code=$?
  set -e
  [ "$code" -eq "$expected_code" ] || { printf '%s\n' "$output"; echo "capacity eval exit=$code want=$expected_code" >&2; exit 1; }
  grep -q "^CAPACITY_STATUS=${expected_status}$" <<<"$output"
  grep -q "$expected_issue" <<<"$output"
}

base=(
  CAPACITY_TEST_MEM_AVAILABLE_PCT=40
  CAPACITY_TEST_SWAP_TOTAL_GIB=2
  CAPACITY_TEST_SWAP_USED_PCT=2
  CAPACITY_TEST_DISK_USED_PCT=30
  CAPACITY_TEST_CONTAINER_MEMORY_PCT=50
  CAPACITY_TEST_RESTARTS=0
  CAPACITY_TEST_OOM=0
  CAPACITY_TEST_STALE_RUNS=0
  CAPACITY_TEST_UNBOUNDED=0
  CAPACITY_TEST_BAD_CONTAINERS=0
  CAPACITY_TEST_DEPLOY_TIMER=active
  CAPACITY_TEST_DEPLOY_RESULT=success
  CAPACITY_TEST_DR_STATUS=disabled
)
run_eval 0 PASS 'ISSUES=none' "${base[@]}"
run_eval 1 WARN 'memory_available_15pct' "${base[@]}" CAPACITY_TEST_MEM_AVAILABLE_PCT=15
run_eval 1 WARN 'container_restarts_1' "${base[@]}" CAPACITY_TEST_RESTARTS=1
run_eval 2 CRITICAL 'container_oom_1' "${base[@]}" CAPACITY_TEST_OOM=1
run_eval 2 CRITICAL 'deploy_timer_inactive' "${base[@]}" CAPACITY_TEST_DEPLOY_TIMER=inactive
run_eval 2 CRITICAL 'dr_status_failed' "${base[@]}" CAPACITY_TEST_DR_STATUS=fail

secret_env=$(mktemp)
compose_out=$(mktemp)
trap 'rm -f "$secret_env" "$compose_out"' EXIT
cat > "$secret_env" <<'ENV'
DB_PASSWORD=capacity-test
REDIS_PASSWORD=capacity-test
JWT_SECRET_KEY=capacity-test
LITELLM_MASTER_KEY=capacity-test
REGISTRY=example.invalid
ENV

docker compose --profile ops -f docker/docker-compose.prod.yml --env-file .env.production --env-file "$secret_env" config > "$compose_out"
# Seven long-running services must have bounded memory; the DR one-shot has its own existing cap.
for bytes in 402653184 100663296 1207959552 201326592 134217728; do
  grep -q "mem_limit: \"$bytes\"" "$compose_out"
done
[ "$(grep -c 'mem_limit:' "$compose_out")" -eq 8 ]
[ "$(grep -c 'mem_reservation:' "$compose_out")" -eq 7 ]
# Every service, including the ops-only DR runner, gets bounded json-file logs.
[ "$(grep -c 'max-size: 10m' "$compose_out")" -eq 9 ]
[ "$(grep -c 'max-file: \"3\"' "$compose_out")" -eq 9 ]

# Cleanup is intentionally conservative: no all-image prune and no volume prune.
grep -q 'docker image prune -f' scripts/production-capacity-status.sh
! grep -q 'docker image prune -a' scripts/production-capacity-status.sh
! grep -q 'docker volume prune' scripts/production-capacity-status.sh

grep -q 'install-production-capacity.sh" --swap-only' scripts/production-deploy-watch.sh
grep -q 'install-production-capacity.sh" --swap-only' scripts/setup-server.sh
grep -q 'production-capacity-status.sh' docker/Dockerfile.runtime

echo CAPACITY_POLICY_VALIDATION=PASS
