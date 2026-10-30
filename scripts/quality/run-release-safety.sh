#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Main-only release/supply-chain checks omitted by Web/Server/AI/Contracts.
# The old validate-repo.sh remains a local diagnostic; Main uses parallel owners.
pnpm quality:capacity
bash -n scripts/staging-slim-watch.sh scripts/install-staging-slim-watch.sh \
  scripts/setup-server.sh scripts/production-deploy-watch.sh scripts/production-postgres18-reset.sh \
  scripts/setup-postgres18-client-wrappers.sh \
  scripts/production-offhost-backup.sh scripts/restore-production-backup.sh \
  scripts/validate-offhost-dr-unit.sh scripts/validate-offhost-dr.sh scripts/install-production-dr.sh scripts/validate-production-dr-installer.sh scripts/validate-production-proxy.sh scripts/validate-supply-chain.sh scripts/test-validate-supply-chain.sh \
  scripts/v2-release-dry-run.sh scripts/test-v2-release-dry-run.sh scripts/schema/check-snapshot.sh
bash scripts/validate-production-capacity.sh
bash scripts/validate-production-postgres18.sh
bash scripts/validate-production-dr-installer.sh
bash scripts/validate-production-proxy.sh
bash scripts/test-validate-supply-chain.sh
bash scripts/validate-supply-chain.sh
bash scripts/test-v2-release-dry-run.sh
pnpm test:static-assets
pnpm test:validation-lifecycle
bash scripts/validate-migration-history.sh
python3 scripts/test_offhost_s3.py
bash scripts/validate-offhost-dr-unit.sh
bash scripts/validate-offhost-dr.sh
git diff --check
git diff --cached --check
echo "RELEASE_SAFETY=PASS"
