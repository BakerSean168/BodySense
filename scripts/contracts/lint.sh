#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
"$ROOT/scripts/contracts/bootstrap-tools.sh"

cd "$ROOT"
node scripts/contracts/check-toolchain.mjs
pnpm exec redocly lint tools/contracts/foundation/openapi.yaml
pnpm exec redocly lint packages/contracts/openapi/bodysense.v1.openapi.yaml
node -e 'const fs=require("fs"); JSON.parse(fs.readFileSync("tools/contracts/foundation/schema.json","utf8"));'
(
  cd tools/contracts/foundation/proto
  "$BIN/buf" lint
)
