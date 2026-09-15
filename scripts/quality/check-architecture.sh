#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

node scripts/quality/check-ts-boundaries.mjs
python3 scripts/quality/check_python_boundaries.py
go run scripts/quality/check_go_boundaries.go

echo "ARCHITECTURE_BOUNDARY_POLICY=PASS"
