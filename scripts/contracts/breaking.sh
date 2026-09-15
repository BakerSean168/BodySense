#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
"$ROOT/scripts/contracts/bootstrap-tools.sh"

"$BIN/oasdiff" breaking \
  "$ROOT/tools/contracts/baselines/foundation.openapi.yaml" \
  "$ROOT/tools/contracts/foundation/openapi.yaml" \
  --fail-on ERR >/dev/null

(
  cd "$ROOT/tools/contracts/foundation/proto"
  "$BIN/buf" breaking . --against "$ROOT/tools/contracts/baselines/proto"
)

(
  cd "$ROOT/contracts/internal/agent-runtime"
  "$BIN/buf" breaking . --against "$ROOT/tools/contracts/baselines/runtime-proto"
)
