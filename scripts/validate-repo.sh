#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
bash -n scripts/setup-server.sh scripts/production-deploy-watch.sh \
  scripts/production-offhost-backup.sh scripts/restore-production-backup.sh \
  scripts/validate-offhost-dr-unit.sh scripts/validate-offhost-dr.sh
bash scripts/validate-production-capacity.sh
bash scripts/validate-supply-chain.sh
pnpm test:static-assets
pnpm lint
pnpm typecheck
pnpm test
bash scripts/validate-migration-history.sh
python3 scripts/test_offhost_s3.py
bash scripts/validate-offhost-dr-unit.sh
bash scripts/validate-offhost-dr.sh
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
