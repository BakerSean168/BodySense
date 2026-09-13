#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
"$ROOT/scripts/contracts/bootstrap-tools.sh"

node "$ROOT/scripts/contracts/conformance.mjs"
node "$ROOT/scripts/contracts/check-architecture.mjs"
node "$ROOT/node_modules/typescript7/bin/tsc" -p "$ROOT/tools/contracts/tsconfig.json" --noEmit
go test "$ROOT/tools/contracts/generated/openapi-go/foundation.gen.go"
PYCACHE="$(mktemp -d)"
trap 'rm -rf "$PYCACHE"' EXIT
PYTHONPYCACHEPREFIX="$PYCACHE" python3 -m py_compile "$ROOT/tools/contracts/generated/proto-python/bodysense/foundation/v1/foundation_pb2.py"
(
  cd "$ROOT/tools/contracts/foundation/proto"
  "$BIN/buf" build >/dev/null
)
