#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
"$ROOT/scripts/contracts/bootstrap-tools.sh"

mkdir -p "$ROOT/apps/api/internal/generated/openapi/v1"
"$BIN/oapi-codegen" \
  -config "$ROOT/apps/api/internal/generated/openapi/v1/oapi-codegen.yaml" \
  -o "$ROOT/apps/api/internal/generated/openapi/v1/bodysense.gen.go" \
  "$ROOT/packages/contracts/openapi/bodysense.v1.openapi.yaml"

(
  cd "$ROOT/apps/web"
  pnpm exec orval --config orval.config.ts
)
