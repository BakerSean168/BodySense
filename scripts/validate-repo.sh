#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
pnpm lint
pnpm typecheck
pnpm test
pnpm nx run ai-service:eval:diagnosis
bash scripts/validate-litellm-gateway.sh
pnpm build
git diff --check
git diff --cached --check
echo "REPO_QUALITY=PASS"
