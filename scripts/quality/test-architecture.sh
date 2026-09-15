#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

node --test scripts/quality/check-ts-boundaries.test.mjs scripts/quality/architecture-gate-integration.test.mjs
python3 -m unittest scripts/quality/test_python_boundaries.py
go test scripts/quality/check_go_boundaries.go scripts/quality/check_go_boundaries_test.go

echo "ARCHITECTURE_POLICY_MUTATION_TESTS=PASS"
