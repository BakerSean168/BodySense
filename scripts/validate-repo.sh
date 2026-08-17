#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
pnpm lint
pnpm typecheck
pnpm test
pnpm build
git diff --check
git diff --cached --check
echo "REPO_QUALITY=PASS"
