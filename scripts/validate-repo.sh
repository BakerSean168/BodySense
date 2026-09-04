#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
bash -n scripts/setup-server.sh scripts/production-deploy-watch.sh scripts/production-postgres18-reset.sh \
  scripts/setup-postgres18-client-wrappers.sh \
  scripts/production-offhost-backup.sh scripts/restore-production-backup.sh \
  scripts/validate-offhost-dr-unit.sh scripts/validate-offhost-dr.sh scripts/install-production-dr.sh scripts/validate-production-dr-installer.sh scripts/validate-supply-chain.sh scripts/test-validate-supply-chain.sh
bash scripts/validate-production-capacity.sh
bash scripts/validate-production-postgres18.sh
bash scripts/validate-production-dr-installer.sh
bash scripts/test-validate-supply-chain.sh
bash scripts/validate-supply-chain.sh
pnpm test:static-assets
pnpm test:delivery
pnpm lint
pnpm typecheck
pnpm test
bash scripts/validate-migration-history.sh
python3 scripts/test_offhost_s3.py
bash scripts/validate-offhost-dr-unit.sh
bash scripts/validate-offhost-dr.sh
pnpm nx run ai-service:eval:assessment-evidence-contract
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
