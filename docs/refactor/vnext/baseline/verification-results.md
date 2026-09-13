# BodySense vNext Phase 00 — Verification Results

Base: `1b649ef43b80d6486d1127da706cbf4bfd7bf2f8` (`refactor/bodysense-vnext`)

## Baseline capture

- `pnpm refactor:vnext:baseline` — PASS.
- `pnpm refactor:vnext:schema` — PASS on disposable `pgvector/pgvector:pg18`.
- Fresh DB migration: `FULL_UP=PASS version=62`.
- Latest migration reversibility: `LATEST_DOWN=PASS version=61` and `LATEST_REPLAY_UP=PASS version=62`.
- Resulting current schema: 48 public tables and 677 columns, migration state `62:false`.

A first disposable-DB probe with plain `postgres:18` failed because the image does not provide the required `vector` extension. The reproducible schema command therefore uses the project's actual PG18+pgvector runtime family rather than weakening migrations for the test harness.

## Repository checks

- `pnpm lint` — PASS for contracts, utils, Go API, Python AI and Web.
- Fresh-worktree `pnpm typecheck` initially exposed `BS-Q-TOOL-001`: `ai-service:typecheck` ran without the optional `ocr` dependency even though imported modules use `fitz`/`pytesseract`.
- Phase 00 repaired the target to `uv run --extra ocr pyright src`; after deleting `.venv`, `pnpm typecheck` — PASS, Python `0 errors, 0 warnings`, contracts/Web typecheck PASS.
- `pnpm test` — PASS:
  - public contract parser: 12 tests;
  - Web: 43 files / 212 tests;
  - Python: 475 tests;
  - Go: `go test ./...` all packages green.
- `pnpm build` — PASS for Go API and Web production build.

## Recorded non-blocking baseline warnings

These are not Phase 00 failures and are tracked in `finding-ledger.json`:

- `BS-Q-TOOL-002`: Nx reports `@nx/eslint:lint` as deprecated for a future Nx v24 migration.
- `BS-Q-TEST-002`: passing Web tests contain repeated `127.0.0.1:3000 ECONNREFUSED` diagnostics plus one React `act(...)` warning.
- `BS-Q-BUILD-001`: Vite reports the BodyExplorer3D production chunk at roughly 1.23 MB before gzip, above the 500 kB warning threshold.
