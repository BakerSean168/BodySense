#!/usr/bin/env bash
# One direct-dev service per tmux window. Infrastructure is managed separately.
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"
# Process Compose starts services independently: the legacy dev-runtime.sh
# preflight and credential filtering must not be bypassed by this entrypoint.
"$ROOT/scripts/dev-litellm-gateway-preflight.sh"

exec_scoped_service() {
  local role=$1
  shift
  local -a process_env=(env -u LITELLM_MASTER_KEY -u PRIMARY_LLM_API_KEY
                        -u OPENROUTER_API_KEY -u GROQ_API_KEY
                        -u BODYSENSE_LITELLM_CLIENT_KEY)
  if [[ "$role" != ai || -z "${LITELLM_API_KEY:-}" || "$LITELLM_API_KEY" == "$LITELLM_MASTER_KEY" ]]; then
    process_env+=(-u LITELLM_API_KEY)
  fi
  case "$role" in
    ai) process_env+=(-u REDIS_PASSWORD -u JWT_SECRET_KEY) ;;
    document|web)
      process_env+=(-u DB_PASSWORD -u DATABASE_URL -u LANGGRAPH_CHECKPOINTER_URL
                    -u REDIS_PASSWORD -u JWT_SECRET_KEY)
      ;;
  esac
  exec "${process_env[@]}" "$@"
}

case "${1:-}" in
  web)
    cd "$ROOT"
    exec_scoped_service web pnpm exec vite --config apps/web/vite.config.ts --host 127.0.0.1 --port "$WEB_PORT" --strictPort
    ;;
  api)
    cd "$ROOT/apps/api"
    if ! command -v air >/dev/null 2>&1; then
      echo "Air is required for Go auto-restart: GOBIN=\$HOME/.local/bin go install github.com/air-verse/air@v1.67.4" >&2
      exit 127
    fi
    exec_scoped_service api air -c .air.toml
    ;;
  ai)
    cd "$ROOT/apps/ai-service"
    run_model_setup uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_pose_model.py
    exec_scoped_service ai uv run --extra ocr --extra pose --extra document-ocr uvicorn src.main:app --reload --reload-dir src --host 127.0.0.1 --port "$AI_SERVICE_PORT"
    ;;
  document)
    cd "$ROOT/apps/ai-service"
    run_model_setup uv run --extra document-ocr python scripts/ensure_health_document_models.py
    exec_scoped_service document uv run --extra document-ocr uvicorn src.document_main:app --reload --reload-dir src --host 127.0.0.1 --port "$DOCUMENT_SERVICE_PORT"
    ;;
  *)
    echo "usage: $0 {web|api|ai|document}" >&2
    exit 2
    ;;
esac
