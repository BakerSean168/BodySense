#!/usr/bin/env bash
# Fails when tools/schema/database-schema.json no longer matches
# the schema produced by applying 000001_vnext_baseline to a throwaway PG18.
# Regenerate with `pnpm schema:snapshot` after editing the baseline in place.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMMITTED="$ROOT/tools/schema/database-schema.json"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

BODYSENSE_SCHEMA_SNAPSHOT_OUT="$tmp/database-schema.json" bash "$ROOT/scripts/schema/capture-current.sh"
if ! diff -u "$COMMITTED" "$tmp/database-schema.json" > "$tmp/schema.diff"; then
  head -200 "$tmp/schema.diff" >&2
  echo 'SCHEMA_SNAPSHOT=DRIFT: run `pnpm schema:snapshot` and commit the result' >&2
  exit 1
fi
echo 'SCHEMA_SNAPSHOT=PASS'
