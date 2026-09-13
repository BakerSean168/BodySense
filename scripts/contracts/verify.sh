#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
pnpm contracts:lint
pnpm contracts:check-generated
pnpm contracts:breaking
pnpm contracts:mutation
pnpm contracts:conformance

# Phase 02 migration coverage is deterministic even before it reaches 100%.
pnpm contracts:route-coverage:check
