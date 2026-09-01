#!/usr/bin/env bash
set -euo pipefail
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

project="bodysense-validator"
export COMPOSE_PROJECT_NAME="$project"

pick_port() {
  python3 - "$1" <<'PY'
import socket
import sys

preferred = int(sys.argv[1])
for candidate in (preferred, 0):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        try:
            sock.bind(("127.0.0.1", candidate))
        except OSError:
            continue
        print(sock.getsockname()[1])
        break
else:
    raise SystemExit("unable to allocate validator port")
PY
}

export DB_PORT="${VALIDATOR_DB_PORT:-$(pick_port 55432)}"
export REDIS_PORT="${VALIDATOR_REDIS_PORT:-$(pick_port 56379)}"
export LITELLM_PORT="${VALIDATOR_LITELLM_PORT:-$(pick_port 0)}"
export API_PORT="${VALIDATOR_API_PORT:-$(pick_port 18080)}"
export AI_SERVICE_PORT="${VALIDATOR_AI_PORT:-$(pick_port 18100)}"
export WEB_PORT="${VALIDATOR_WEB_PORT:-$(pick_port 15173)}"

cleanup() {
  if [[ "${KEEP_VALIDATOR_STACK:-0}" != "1" ]]; then
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

wait_http() {
  local url="$1"
  local name="$2"
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "$name=PASS"
      return 0
    fi
    sleep 2
  done
  echo "$name=FAIL" >&2
  return 1
}

if [[ "${SKIP_QUALITY:-0}" != "1" ]]; then
  # Repository quality must run before any production-shaped runtime variables
  # are exported. Several hermetic tests intentionally verify their own secret
  # precedence; leaking the validator DB password into those tests breaks the
  # trust boundary they are designed to prove.
  bash scripts/validate-repo.sh
fi

# Runtime-only environment starts here. Historical rollout behavior remains
# covered by unit/promotion-policy tests; this disposable stack proves clean
# environments serve the current Champions without operator overrides.
export DIAGNOSIS_CHAMPION_CONFIGURATION_ID="diag-config-5a4a13627e14b4cf"
export DIAGNOSIS_CHALLENGER_CONFIGURATION_ID=""
export DIAGNOSIS_ROLLBACK_CONFIGURATION_ID="diag-config-f492eb1c0c6676ae"
export DIAGNOSIS_ROLLOUT_STAGE="champion"
export DIAGNOSIS_PROMOTION_RECORD=""
export TREATMENT_CHAMPION_CONFIGURATION_ID="treat-config-f68eec9846664596"
export TREATMENT_CHALLENGER_CONFIGURATION_ID=""
export TREATMENT_ROLLBACK_CONFIGURATION_ID="treat-config-85718f8e90ac9d80"
export TREATMENT_ROLLOUT_STAGE="champion"
export TREATMENT_PROMOTION_RECORD=""
export DB_USER="bodysense"
export DB_PASSWORD="bodysense123"
export DB_NAME="bodysense"
export REDIS_PASSWORD="bodysense123"
export JWT_SECRET_KEY="bodysense-local-validator-secret"
compose=(docker compose -f docker/docker-compose.yml --profile dev)

# Only the production-shaped runtime/E2E phase is stubbed/deterministic. Quality
# tests above observed their normal model/gateway contracts and owned their mocks.
export BODYSENSE_E2E_STUB_AI=1
export BODYSENSE_DETERMINISTIC_AI="true"

"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
"${compose[@]}" build api ai-service web
"${compose[@]}" up -d postgres-dev redis-dev ai-service api web

wait_http "http://127.0.0.1:${API_PORT}/api/health" "API_HEALTH"
wait_http "http://127.0.0.1:${AI_SERVICE_PORT}/health" "AI_HEALTH"
wait_http "http://127.0.0.1:${WEB_PORT}" "WEB_HEALTH"

migration_db="bodysense_migration_validator"
"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS ${migration_db} WITH (FORCE);" >/dev/null
"${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${migration_db};" >/dev/null
(
  cd apps/api
  go run ./cmd/migration-validator \
    -database-url "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${migration_db}?sslmode=disable" \
    -migrations "file://migrations"
)
(
  cd apps/api
  go run ./cmd/domain-validator \
    -database-url "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${migration_db}?sslmode=disable"
)

bash scripts/validate-knowledge-publication.sh

E2E_BASE_URL="http://127.0.0.1:${WEB_PORT}" \
E2E_API_BASE_URL="http://127.0.0.1:${API_PORT}" \
E2E_RESTART_API_COMMAND="$repo_root/scripts/e2e-expire-run-and-restart-api.sh" \
pnpm e2e

diagnosis_latest_analyses="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM diagnosis_analyses WHERE agent_configuration_id='diag-config-5a4a13627e14b4cf';")"
diagnosis_legacy_analyses="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM diagnosis_analyses WHERE agent_configuration_id='diag-config-f492eb1c0c6676ae';")"
diagnosis_rollout_observations="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM diagnosis_rollout_observations;")"
if [[ "$diagnosis_latest_analyses" -lt 1 || "$diagnosis_legacy_analyses" -ne 0 || "$diagnosis_rollout_observations" -ne 0 ]]; then
  echo "DIAGNOSIS_BASELINE_VALIDATION=FAIL latest=${diagnosis_latest_analyses} legacy=${diagnosis_legacy_analyses} rollout_observations=${diagnosis_rollout_observations}" >&2
  exit 1
fi
echo "DIAGNOSIS_BASELINE_VALIDATION=PASS latest=${diagnosis_latest_analyses} legacy=${diagnosis_legacy_analyses} rollout_observations=${diagnosis_rollout_observations}"

treatment_latest_revisions="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM treatment_revisions WHERE agent_configuration_id='treat-config-f68eec9846664596';")"
treatment_legacy_revisions="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM treatment_revisions WHERE agent_configuration_id='treat-config-85718f8e90ac9d80';")"
treatment_rollout_observations="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM treatment_rollout_observations;")"
if [[ "$treatment_latest_revisions" -lt 1 || "$treatment_legacy_revisions" -ne 0 || "$treatment_rollout_observations" -ne 0 ]]; then
  echo "TREATMENT_BASELINE_VALIDATION=FAIL latest=${treatment_latest_revisions} legacy=${treatment_legacy_revisions} rollout_observations=${treatment_rollout_observations}" >&2
  exit 1
fi
echo "TREATMENT_BASELINE_VALIDATION=PASS latest=${treatment_latest_revisions} legacy=${treatment_legacy_revisions} rollout_observations=${treatment_rollout_observations}"

treatment_decision_traces="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM treatment_revisions WHERE generation_decision_trace <> '{}'::jsonb AND acceptance_state='accepted' AND acceptance_decision_trace <> '{}'::jsonb;")"
if [[ "$treatment_decision_traces" -lt 1 ]]; then
  echo "TREATMENT_DECISION_TRACE_VALIDATION=FAIL accepted_traces=${treatment_decision_traces}" >&2
  exit 1
fi
echo "TREATMENT_DECISION_TRACE_VALIDATION=PASS accepted_traces=${treatment_decision_traces}"

treatment_replay_inputs="$("${compose[@]}" exec -T postgres-dev psql -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT count(*) FROM treatment_revisions WHERE replay_input <> '{}'::jsonb;")"
if [[ "$treatment_replay_inputs" -lt 1 ]]; then
  echo "TREATMENT_REPLAY_INPUT_VALIDATION=FAIL replay_inputs=${treatment_replay_inputs}" >&2
  exit 1
fi
echo "TREATMENT_REPLAY_INPUT_VALIDATION=PASS replay_inputs=${treatment_replay_inputs}"

echo "LOCAL_DEPLOY_VALIDATION=PASS"
