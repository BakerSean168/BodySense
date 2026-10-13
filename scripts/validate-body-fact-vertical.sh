#!/usr/bin/env bash
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
EVIDENCE="$ROOT/.runtime/schema-first-v2-evidence/vertical-e2e-$$"
mkdir -p "$EVIDENCE/uploads"
PG="bodysense-v2-vertical-pg-$$"
REDIS="bodysense-v2-vertical-redis-$$"
API_PID=""
AI_PID=""
API_PORT="${BODY_FACT_TEST_API_PORT:-28371}"
AI_PORT="${BODY_FACT_TEST_AI_PORT:-28372}"
WEB_PORT="${BODY_FACT_TEST_WEB_PORT:-28373}"
PG_IMAGE="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
REDIS_IMAGE="redis:8.10.2-alpine@sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0"
cleanup() {
  for pid in "$API_PID" "$AI_PID"; do
    if test -n "$pid"; then kill "$pid" 2>/dev/null || true; fi
  done
  wait 2>/dev/null || true
  docker rm -f "$PG" "$REDIS" >/dev/null 2>&1 || true
  printf 'BODY_FACT_VERTICAL_RESOURCES=CLEANED\nEVIDENCE=%s\n' "$EVIDENCE"
}
trap cleanup EXIT
trap 'code=$?; echo "VERTICAL_TEST_FAILURE line=$LINENO status=$code" >&2; docker logs --tail 30 "$PG" 2>&1 || true; test ! -f "$EVIDENCE/api.log" || tail -25 "$EVIDENCE/api.log"; test ! -f "$EVIDENCE/ai.log" || tail -25 "$EVIDENCE/ai.log"' ERR
for port in "$API_PORT" "$AI_PORT" "$WEB_PORT"; do
  if ss -ltnH | awk '{print $4}' | grep -Eq ":${port}$"; then
    echo "Test port $port already in use; no existing service will be stopped." >&2
    exit 1
  fi
done

docker run -d --name "$PG" --label bodysense.purpose=synthetic-v2-test \
  -e POSTGRES_PASSWORD=synthetic-vertical-test -e POSTGRES_DB=bodysense_v2_test "$PG_IMAGE" >/dev/null
docker run -d --name "$REDIS" --label bodysense.purpose=synthetic-v2-test "$REDIS_IMAGE" >/dev/null
for _ in $(seq 1 45); do
  if docker exec "$PG" pg_isready -h 127.0.0.1 -U postgres -d bodysense_v2_test >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$PG" pg_isready -h 127.0.0.1 -U postgres -d bodysense_v2_test >/dev/null
PG_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PG")"
REDIS_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$REDIS")"
(
  cd "$ROOT/apps/api"
  GOMAXPROCS=2 go build -p 2 -o "$EVIDENCE/api" ./cmd/server
)
export API_PORT API_HOST=127.0.0.1 APP_ENV=test
export DB_HOST="$PG_IP" DB_PORT=5432 DB_NAME=bodysense_v2_test DB_USER=postgres DB_PASSWORD=synthetic-vertical-test DB_SSLMODE=disable
export REDIS_HOST="$REDIS_IP" REDIS_PORT=6379 REDIS_PASSWORD=""
export JWT_SECRET_KEY=bodysense-isolated-v2-e2e-only-secret
export CORS_ORIGINS="http://127.0.0.1:$WEB_PORT"
export AI_SERVICE_URL="http://127.0.0.1:$AI_PORT"
export UPLOAD_STORAGE_BACKEND=local UPLOAD_STORAGE_LOCAL_ROOT="$EVIDENCE/uploads"
export CONSULTATION_CHAMPION_CONFIGURATION_ID=consult-config-83b6a1cc70477c82
export ENVIRONMENT=e2e BODYSENSE_E2E_STUB_AI=1 BODYSENSE_DETERMINISTIC_AI=true
export DATABASE_URL="postgresql://postgres:synthetic-vertical-test@$PG_IP:5432/bodysense_v2_test"
(
  cd "$ROOT/apps/api"
  exec "$EVIDENCE/api"
) >"$EVIDENCE/api.log" 2>&1 &
API_PID=$!
(
  cd "$ROOT/apps/ai-service"
  exec .venv/bin/python -m uvicorn src.main:app --host 127.0.0.1 --port "$AI_PORT"
) >"$EVIDENCE/ai.log" 2>&1 &
AI_PID=$!
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$API_PORT/api/health" >/dev/null 2>&1 && \
     curl -fsS "http://127.0.0.1:$AI_PORT/health" >/dev/null 2>&1; then break; fi
  if ! kill -0 "$API_PID" 2>/dev/null || ! kill -0 "$AI_PID" 2>/dev/null; then
    tail -35 "$EVIDENCE/api.log" "$EVIDENCE/ai.log"
    exit 1
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:$API_PORT/api/health" >/dev/null
curl -fsS "http://127.0.0.1:$AI_PORT/health" >/dev/null
cd "$ROOT"
CI=true E2E_BASE_URL="http://127.0.0.1:$WEB_PORT" E2E_API_BASE_URL="http://127.0.0.1:$API_PORT" \
  timeout 720 node node_modules/@playwright/test/cli.js test \
  apps/web/e2e/ask-user-body-facts.spec.ts apps/web/e2e/body-fact-records.spec.ts apps/web/e2e/longitudinal-full-loop.spec.ts apps/web/e2e/longitudinal-health.spec.ts apps/web/e2e/lifestyle-current-update.spec.ts \
  --reporter=list | tee "$EVIDENCE/browser.log"
echo BODY_FACT_VERTICAL_E2E=PASS
