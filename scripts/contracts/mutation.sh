#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
"$ROOT/scripts/contracts/bootstrap-tools.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cp "$ROOT/tools/contracts/foundation/openapi.yaml" "$TMP/openapi-breaking.yaml"
python3 - "$TMP/openapi-breaking.yaml" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
old = """        revision:\n          type: integer\n          minimum: 0\n"""
new = """        revision:\n          type: string\n"""
if old not in s:
    raise SystemExit("OpenAPI mutation anchor missing")
p.write_text(s.replace(old, new, 1))
PY
if "$BIN/oasdiff" breaking "$ROOT/tools/contracts/foundation/openapi.yaml" "$TMP/openapi-breaking.yaml" --fail-on ERR >/dev/null 2>&1; then
  echo "expected OpenAPI breaking mutation to be rejected" >&2
  exit 1
fi

cp "$ROOT/tools/contracts/foundation/openapi.yaml" "$TMP/openapi-additive.yaml"
python3 - "$TMP/openapi-additive.yaml" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
anchor = """        revision:\n          type: integer\n          minimum: 0\n"""
addition = anchor + """        additive_probe:\n          type: string\n"""
if anchor not in s:
    raise SystemExit("OpenAPI additive mutation anchor missing")
p.write_text(s.replace(anchor, addition, 1))
PY
"$BIN/oasdiff" breaking "$ROOT/tools/contracts/foundation/openapi.yaml" "$TMP/openapi-additive.yaml" --fail-on ERR >/dev/null

cp -R "$ROOT/tools/contracts/foundation/proto" "$TMP/proto"
python3 - "$TMP/proto/bodysense/foundation/v1/foundation.proto" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
old = "uint64 expected_revision = 2;"
new = "string expected_revision = 2;"
if old not in s:
    raise SystemExit("Proto mutation anchor missing")
p.write_text(s.replace(old, new, 1))
PY
if (
  cd "$TMP/proto"
  "$BIN/buf" breaking . --against "$ROOT/tools/contracts/foundation/proto"
) >/dev/null 2>&1; then
  echo "expected Proto breaking mutation to be rejected" >&2
  exit 1
fi

node "$ROOT/scripts/contracts/schema-mutation.mjs"
node "$ROOT/scripts/contracts/runtime-proto-mutation.mjs"
