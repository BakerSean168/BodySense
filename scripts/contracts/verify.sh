#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
pnpm contracts:lint
pnpm contracts:check-generated
pnpm contracts:breaking
pnpm contracts:mutation
pnpm contracts:conformance

# Postman is a generated view of the canonical OpenAPI, not a second contract.
pnpm postman:verify
