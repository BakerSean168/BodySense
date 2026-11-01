#!/usr/bin/env bash
set -euo pipefail

# Low-cost Codespace: prepare toolchain only; install dependencies per task.
cd "$(git rev-parse --show-toplevel)"

expected_pnpm=$(node -p 'require("./package.json").packageManager.split("@").pop()')
actual_pnpm=$(pnpm --version)
if [[ "$actual_pnpm" != "$expected_pnpm" ]]; then
  echo "BodySense requires pnpm $expected_pnpm; found $actual_pnpm" >&2
  exit 1
fi
node -e 'if (Number(process.versions.node.split(".")[0]) < 24) process.exit(1)'
echo "BodySense toolchain verified: Node $(node --version), pnpm $actual_pnpm"

case "${1:-}" in
  ""|--check)
    echo 'Lite: Go, Python AI and Docker are intentionally not installed.'
    echo 'Install dependencies as needed: pnpm install --frozen-lockfile'
    ;;
  --full-check)
    command -v go >/dev/null || { echo 'Full requires Go' >&2; exit 1; }
    command -v python3.13 >/dev/null || { echo 'Full requires Python 3.13' >&2; exit 1; }
    command -v docker >/dev/null || { echo 'Full requires Docker CLI' >&2; exit 1; }
    go version | grep -Eq 'go1\.26(\.|[[:space:]])' || { echo 'Full requires Go 1.26' >&2; exit 1; }
    python3.13 -c 'import sys; assert sys.version_info[:2] == (3,13)'
    echo "Full toolchains verified: $(go version), $(python3.13 --version)"
    ;;
  --install)
    pnpm install --frozen-lockfile
    ;;
  *)
    echo 'Usage: bash .devcontainer/post-create.sh [--check|--full-check|--install]' >&2
    exit 2
    ;;
esac
