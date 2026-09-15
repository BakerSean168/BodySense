#!/usr/bin/env bash
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
MIGRATIONS="$ROOT/apps/api/migrations"
MANIFEST="$MIGRATIONS/checksums.sha256"

python3 - "$MIGRATIONS" <<'PY'
from pathlib import Path
from collections import defaultdict
import sys

root = Path(sys.argv[1])
versions: dict[int, set[str]] = defaultdict(set)
for path in root.glob("*.sql"):
    prefix = path.name.split("_", 1)[0]
    if not prefix.isdigit():
        continue
    direction = path.name.rsplit(".", 2)[-2]
    versions[int(prefix)].add(direction)

if not versions:
    raise SystemExit("no migrations found")
latest = max(versions)
missing = [v for v in range(1, latest + 1) if v not in versions]
incomplete = {v: sorted(parts) for v, parts in versions.items() if parts != {"up", "down"}}
if missing:
    raise SystemExit(f"unexpected migration version gaps: {missing}")
if incomplete:
    raise SystemExit(f"migration up/down pair incomplete: {incomplete}")
if not (root / "000001_vnext_baseline.up.sql").is_file() or not (root / "000001_vnext_baseline.down.sql").is_file():
    raise SystemExit("vNext migration history must start at 000001_vnext_baseline")
if (root / "baselines" / "production-v29.sql").exists():
    raise SystemExit("historical production-v29 fixture must not remain in the active migration tree")
print(f"MIGRATION_SEQUENCE=PASS latest={latest}")
PY

[ -s "$MANIFEST" ] || { echo "missing migration checksum manifest: $MANIFEST" >&2; exit 1; }
(
  cd "$MIGRATIONS"
  sha256sum -c checksums.sha256
)

expected=$(mktemp)
actual=$(mktemp)
trap 'rm -f "$expected" "$actual"' EXIT
sed -E 's/^[0-9a-f]+  //' "$MANIFEST" | sort > "$expected"
find "$MIGRATIONS" -maxdepth 1 -type f -name '*.sql' -printf '%f\n' | sort > "$actual"
if ! diff -u "$expected" "$actual"; then
  echo "migration checksum manifest does not cover the exact SQL migration set" >&2
  exit 1
fi

echo MIGRATION_IMMUTABILITY=PASS
