#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

bash -n scripts/setup-server.sh scripts/production-deploy-watch.sh

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'ENV'
DB_PASSWORD=test-db-password
REDIS_PASSWORD=test-redis-password
JWT_SECRET_KEY=test-jwt-secret
LITELLM_MASTER_KEY=test-litellm-key
OPENROUTER_API_KEY=test-openrouter-key
EMBEDDING_API_KEY=test-embedding-key
MIMO_API_KEY=
BODYSENSE_EDGE_MODE=external
ENV

docker compose -p bodysense-runtime-validate \
  -f docker/docker-compose.prod.yml \
  --env-file .env.production \
  --env-file "$tmp" config -q

rendered=$(docker compose -p bodysense-runtime-validate \
  -f docker/docker-compose.prod.yml \
  --env-file .env.production \
  --env-file "$tmp" config --format json)
python3 -c '
import json,sys
cfg=json.load(sys.stdin)
ports=cfg["services"]["web"].get("ports") or []
if not any(str(p.get("host_ip")) == "127.0.0.1" and int(p.get("published")) == 18080 and int(p.get("target")) == 80 for p in ports):
    raise SystemExit("external-edge loopback web port is missing from production compose")
' <<<"$rendered"

grep -q 'BODYSENSE_EDGE_MODE=dedicated' .env.production
grep -q 'external edge mode active' scripts/production-deploy-watch.sh

echo 'PRODUCTION_RUNTIME_VALIDATE=PASS'
