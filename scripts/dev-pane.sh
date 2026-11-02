#!/usr/bin/env bash
# One direct-dev service per tmux window. Infrastructure is managed separately.
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"

case "${1:-}" in
  web)
    cd "$ROOT"
    exec pnpm exec vite --config apps/web/vite.config.ts --host 127.0.0.1 --port "$WEB_PORT" --strictPort
    ;;
  api)
    cd "$ROOT/apps/api"
    if ! command -v air >/dev/null 2>&1; then
      echo "Air is required for Go auto-restart: GOBIN=\$HOME/.local/bin go install github.com/air-verse/air@v1.67.4" >&2
      exit 127
    fi
    exec air -c .air.toml
    ;;
  ai)
    cd "$ROOT/apps/ai-service"
    uv run --extra ocr --extra pose --extra document-ocr python scripts/ensure_pose_model.py
    exec uv run --extra ocr --extra pose --extra document-ocr uvicorn src.main:app --reload --reload-dir src --host 127.0.0.1 --port "$AI_SERVICE_PORT"
    ;;
  document)
    cd "$ROOT/apps/ai-service"
    uv run --extra document-ocr python scripts/ensure_health_document_models.py
    exec uv run --extra document-ocr uvicorn src.document_main:app --reload --reload-dir src --host 127.0.0.1 --port "$DOCUMENT_SERVICE_PORT"
    ;;
  *)
    echo "usage: $0 {web|api|ai|document}" >&2
    exit 2
    ;;
esac
