#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
bash -n scripts/setup-server.sh scripts/production-deploy-watch.sh
pnpm lint
pnpm typecheck
pnpm test
bash scripts/validate-migration-history.sh
pnpm nx run ai-service:eval:diagnosis
pnpm nx run ai-service:eval:diagnosis-evidence-policy
pnpm nx run ai-service:eval:diagnosis-promotion
pnpm nx run ai-service:eval:treatment
pnpm nx run ai-service:eval:treatment-evidence-gap
pnpm nx run ai-service:eval:treatment-promotion
bash scripts/validate-litellm-gateway.sh
pnpm build
git diff --check
git diff --cached --check
echo "REPO_QUALITY=PASS"
