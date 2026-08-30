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
  --profile dev
)
services=(postgres-dev redis-dev litellm-gateway)

up() {
  "${compose[@]}" up -d --wait --wait-timeout 120 "${services[@]}"
  status
}

status() {
  echo "BodySense persistent dev infra: postgres=${DB_PORT} redis=${REDIS_PORT} litellm=${LITELLM_PORT}"
  "${compose[@]}" ps "${services[@]}"
}

down() {
  "${compose[@]}" down --remove-orphans
}

logs() {
  "${compose[@]}" logs -f --tail "${1:-120}" "${services[@]}"
}

case "${1:-status}" in
  up) up ;;
  status) status ;;
  restart) down; up ;;
  down) down ;;
  logs) shift; logs "${1:-120}" ;;
  *)
    echo "usage: $0 {up|status|restart|down|logs [lines]}" >&2
    exit 2
    ;;
esac
