#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"

COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-bodysense-dev-infra}
export COMPOSE_PROJECT_NAME
compose=(
  docker compose
  -f "$ROOT/docker/docker-compose.yml"
  -f "$ROOT/docker/docker-compose.dev-infra.yml"
)
# DEV-ONLY: append the Oracle2 overlay only when the opt-in resolved it.
if [[ -n "${BODYSENSE_DEV_COMPOSE_OVERLAY:-}" ]]; then
  compose+=(-f "$BODYSENSE_DEV_COMPOSE_OVERLAY")
fi
compose+=(--profile dev)
services=(postgres-dev redis-dev litellm-gateway)

up() {
  "${compose[@]}" up -d --build --wait --wait-timeout 120 "${services[@]}"
  status
}

status() {
  echo "BodySense persistent dev infra: postgres=${DB_PORT} redis=${REDIS_PORT} litellm=${LITELLM_PORT} oracle2_dev_opt_in=${BODYSENSE_LITELLM_DEV_ORACLE2}"
  "${compose[@]}" ps "${services[@]}"
}

down() {
  "${compose[@]}" down --remove-orphans
}

# Recreate only the local LiteLLM gateway (for example after a Tailnet IP change).
# Never down the shared dev Compose project or recreate postgres-dev/redis-dev.
restart_gateway() {
  "${compose[@]}" up -d --no-deps --build --force-recreate --wait --wait-timeout 120 litellm-gateway
  status
}

logs() {
  "${compose[@]}" logs -f --tail "${1:-120}" "${services[@]}"
}

# Supplier identity preflight is read-only and runs before any Compose mutation.
# A shared, unqualified upstream could silently select a different supplier.
# Status/down/logs remain available for recovering already-running dev services.
oracle2_route_preflight() {
  if [[ -n "${BODYSENSE_DEV_COMPOSE_OVERLAY:-}" ]]; then
    python3 "$ROOT/scripts/dev-oracle2-route-preflight.py" \
      "$ROOT/docker/litellm/config.dev-oracle2.yaml"
  fi
}

case "${1:-status}" in
  up) oracle2_route_preflight; up ;;
  status) status ;;
  restart) oracle2_route_preflight; down; up ;;
  restart-gateway) oracle2_route_preflight; restart_gateway ;;
  down) down ;;
  logs) shift; logs "${1:-120}" ;;
  *)
    echo "usage: $0 {up|status|restart|restart-gateway|down|logs [lines]}" >&2
    exit 2
    ;;
esac
