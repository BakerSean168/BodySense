#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ENV_FILE=${BODYSENSE_DEV_ENV_FILE:-$ROOT/.env.dev.local}

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

# BodySense direct-dev owns the 20100-20199 project block. The local env file
# may override these values; otherwise the canonical project allocation wins.
export DEV_BIND_HOST=${DEV_BIND_HOST:-127.0.0.1}
export BODYSENSE_WEB_PORT=${BODYSENSE_WEB_PORT:-${WEB_PORT:-20100}}
export WEB_PORT=${WEB_PORT:-$BODYSENSE_WEB_PORT}
export API_HOST=${API_HOST:-127.0.0.1}
export API_PORT=${API_PORT:-20101}
export AI_SERVICE_PORT=${AI_SERVICE_PORT:-20102}
export DOCUMENT_SERVICE_PORT=${DOCUMENT_SERVICE_PORT:-20103}
export DB_HOST=${DB_HOST:-127.0.0.1}
export DB_PORT=${DB_PORT:-20110}
export REDIS_HOST=${REDIS_HOST:-127.0.0.1}
export REDIS_PORT=${REDIS_PORT:-20111}
export LITELLM_PORT=${LITELLM_PORT:-20112}

export DB_USER=${DB_USER:-bodysense}
export DB_PASSWORD=${DB_PASSWORD:-bodysense123}
export DB_NAME=${DB_NAME:-bodysense}
export DB_SSLMODE=${DB_SSLMODE:-disable}
export REDIS_PASSWORD=${REDIS_PASSWORD:-bodysense123}
export JWT_SECRET_KEY=${JWT_SECRET_KEY:-bodysense-dev-only-secret}
export JWT_ACCESS_TTL_HOURS=${JWT_ACCESS_TTL_HOURS:-168}
export JWT_REFRESH_TTL_HOURS=${JWT_REFRESH_TTL_HOURS:-720}
export TRUSTED_PROXIES=${TRUSTED_PROXIES:-127.0.0.1,::1}
export LITELLM_MASTER_KEY=${LITELLM_MASTER_KEY:-sk-bodysense-dev-gateway}
export LITELLM_BASE_URL=${LITELLM_BASE_URL:-http://127.0.0.1:${LITELLM_PORT}/v1}
# The offline gateway master credential is never an application client key.
# Refuse ambient/legacy LITELLM_API_KEY without a BodySense-scoped source;
# otherwise a shared gateway key could silently become this project's key.
if [[ -n "${LITELLM_API_KEY:-}" && -z "${BODYSENSE_LITELLM_CLIENT_KEY:-}" ]]; then
  if [[ "$LITELLM_API_KEY" == "$LITELLM_MASTER_KEY" ]]; then
    echo "BodySense LiteLLM client credential BLOCKED: master-key alias is forbidden" >&2
  else
    echo "BodySense LiteLLM client credential BLOCKED: unscoped legacy LITELLM_API_KEY" >&2
  fi
  exit 1
fi
# Leave inference unavailable until a separately authorized client key exists.
export LITELLM_API_KEY=${BODYSENSE_LITELLM_CLIENT_KEY:-}
export DATABASE_URL=${DATABASE_URL:-postgresql://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${DB_NAME}}
export AI_SERVICE_URL=${AI_SERVICE_URL:-http://127.0.0.1:${AI_SERVICE_PORT}}
export HEALTH_DOCUMENT_SERVICE_URL=${HEALTH_DOCUMENT_SERVICE_URL:-http://127.0.0.1:${DOCUMENT_SERVICE_PORT}}
export EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER:-hashing}
export CORS_ORIGINS=${CORS_ORIGINS:-http://localhost:${WEB_PORT},http://127.0.0.1:${WEB_PORT}}
export VITE_DEV_API_TARGET=${VITE_DEV_API_TARGET:-http://127.0.0.1:${API_PORT}}
export VITE_WS_URL=${VITE_WS_URL:-ws://127.0.0.1:${API_PORT}/ws}
# Persistent GCP direct-dev is normally reached over Tailscale/SSH, where RTT
# makes Vite's native-ESM module waterfall expensive. Bundled Dev preserves HMR
# while collapsing that browser-facing request graph. Set false in
# .env.dev.local when debugging Vite/plugin behavior with classic dev serving.
export BODYSENSE_VITE_BUNDLED_DEV=${BODYSENSE_VITE_BUNDLED_DEV:-true}

# Model asset preparation needs neither gateway/provider nor application credentials.
# Keep these out of uv and any helper it launches, even during interactive start.
run_model_setup() {
  env -u LITELLM_MASTER_KEY -u LITELLM_API_KEY -u BODYSENSE_LITELLM_CLIENT_KEY \
      -u PRIMARY_LLM_API_KEY -u OPENROUTER_API_KEY -u GROQ_API_KEY \
      -u DB_PASSWORD -u DATABASE_URL -u LANGGRAPH_CHECKPOINTER_URL \
      -u REDIS_PASSWORD -u JWT_SECRET_KEY "$@"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  if [[ $# -eq 0 ]]; then
    printf 'BodySense direct-dev env: web=%s api=%s ai=%s document=%s postgres=%s redis=%s litellm=%s\n' \
      "$WEB_PORT" "$API_PORT" "$AI_SERVICE_PORT" "$DOCUMENT_SERVICE_PORT" "$DB_PORT" "$REDIS_PORT" "$LITELLM_PORT"
    exit 0
  fi
  cd "$ROOT"
  exec "$@"
fi
