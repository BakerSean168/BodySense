#!/usr/bin/env bash
set -euo pipefail

# Codespaces startup should be deterministic and cheap, not provision all services.
cd "$(git rev-parse --show-toplevel)"

expected_pnpm=$(node -p 'require("./package.json").packageManager.split("@").pop()')
actual_pnpm=$(pnpm --version)
if [[ "$actual_pnpm" != "$expected_pnpm" ]]; then
  echo "BodySense requires pnpm $expected_pnpm; found $actual_pnpm" >&2
  exit 1
fi
node -e 'const [major] = process.versions.node.split(".").map(Number); if (major < 24) process.exit(1)'
echo "BodySense Codespaces Node/pnpm OK: Node $(node --version), pnpm $actual_pnpm"

case "${1:-}" in
  ""|--check)
    echo 'Lite profile: Go, Python AI dependencies and Docker are intentionally not installed.'
    echo 'Run pnpm install --frozen-lockfile when needed.'
    ;;
  --full-check)
    command -v go >/dev/null || { echo 'Full profile missing Go' >&2; exit 1; }
    command -v python3.13 >/dev/null || { echo 'Full profile missing Python 3.13' >&2; exit 1; }
    command -v docker >/dev/null || { echo 'Full profile missing Docker client' >&2; exit 1; }
    go version | grep -Eq 'go1\.26(\.|[[:space:]])' || { echo 'Full profile needs Go 1.26' >&2; exit 1; }
    python3.13 -c 'import sys; assert sys.version_info[:2] == (3,13)'
    echo "BodySense Full toolchains OK: $(go version), $(python3.13 --version)"
    echo 'Run pnpm install --frozen-lockfile, uv sync --frozen, or pnpm dev:infra:up only when needed.'
    ;;
  --install)
    pnpm install --frozen-lockfile
    ;;
  *)
    echo 'Usage: bash .devcontainer/post-create.sh [--check|--full-check|--install]' >&2
    exit 2
    ;;
esac
