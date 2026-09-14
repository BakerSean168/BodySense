#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BEFORE="$(mktemp)"
AFTER="$(mktemp)"
trap 'rm -f "$BEFORE" "$AFTER"' EXIT

snapshot() {
  cd "$ROOT"
  {
    find tools/contracts/generated -type f -print 2>/dev/null
    [[ -f apps/api/internal/generated/openapi/v1/bodysense.gen.go ]] && printf '%s\n' apps/api/internal/generated/openapi/v1/bodysense.gen.go
    find apps/api/internal/generated/runtimeproto -type f -print 2>/dev/null
    find apps/ai-service/src/generated/runtimeproto -type f ! -path '*/__pycache__/*' ! -name '*.pyc' -print 2>/dev/null
    find apps/web/src/generated/api -type f -print 2>/dev/null
    find packages/contracts/generated -type f -print 2>/dev/null
    [[ -f tools/contracts/foundation/proto/buf.lock ]] && printf '%s\n' tools/contracts/foundation/proto/buf.lock
    [[ -f contracts/internal/agent-runtime/buf.lock ]] && printf '%s\n' contracts/internal/agent-runtime/buf.lock
  } | LC_ALL=C sort | while read -r file; do
    sha256sum "$file"
  done
}

snapshot > "$BEFORE"
pnpm --dir "$ROOT" contracts:generate
snapshot > "$AFTER"

if ! diff -u "$BEFORE" "$AFTER"; then
  echo "generated contract artifacts are stale or nondeterministic" >&2
  exit 1
fi
