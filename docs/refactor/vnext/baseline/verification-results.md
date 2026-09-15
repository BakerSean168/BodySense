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

## Recorded non-blocking baseline warnings — closed during vNext

Phase 00 intentionally recorded these as non-blocking findings rather than hiding them. Final merge-readiness closeout has now resolved all three in `finding-ledger.json`:

- `BS-Q-TOOL-002` — Web lint now uses the `@nx/eslint/plugin` inferred target; the deprecated `@nx/eslint:lint` executor is gone and lint is warning-free.
- `BS-Q-TEST-002` — unit tests isolate best-effort client diagnostics and wrap the mounted Zustand reset in `act()`; the full 52-file / 266-test Web suite passes with `ECONNREFUSED=0` and React `act(...)` warnings `=0`.
- `BS-Q-BUILD-001` — BodyExplorer3D remains a lazy dynamic entry with an explicit 1.30 MB raw / 300 kB gzip budget, while every other JS chunk retains a 500 kB raw budget. The current 1,237.65 kB / 286.73 kB gzip viewer build passes without the generic Vite oversize warning.

The original Phase 00 measurements above remain historical baseline evidence; this section records their final disposition rather than rewriting the starting point.
