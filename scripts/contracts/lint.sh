#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
# Keep bootstrap behavior pinned, verifiable and fail-closed before any
# networked tool installation occurs.
"$ROOT/scripts/contracts/test-go-install-retry.sh"
"$ROOT/scripts/contracts/bootstrap-tools.sh"

cd "$ROOT"
node scripts/contracts/check-toolchain.mjs
pnpm exec redocly lint tools/contracts/foundation/openapi.yaml
pnpm exec redocly lint packages/contracts/openapi/bodysense.v1.openapi.yaml
node -e 'const fs=require("fs"); JSON.parse(fs.readFileSync("tools/contracts/foundation/schema.json","utf8"));'
node scripts/contracts/check-stream-event-schema.mjs
(
  cd tools/contracts/foundation/proto
  "$BIN/buf" lint
)
