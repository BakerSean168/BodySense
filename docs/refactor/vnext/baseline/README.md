# BodySense vNext Phase 00 Baseline

This directory is the evidence package for `refactor/vnext-00-baseline`.

## Generated evidence

- `current-system-inventory.json` — public/internal routes, public stream variants, migration lineage, Agent manifests, referenced environment variables and broad code-quality discovery signals.
- `current-system-baseline.md` — human-readable summary generated from the inventory.
- `database-schema.json` — fresh PostgreSQL 18 + pgvector schema after applying migrations through version 62 and replaying the latest down/up migration.

`database-schema.json` is frozen Phase 00 evidence. Do not regenerate it from a later refactor phase: the current canonical schema has its own snapshot under `docs/refactor/vnext/schema/`. Phase 11 retired the one-time Phase 00 inventory-capture script from the active tree; exact reproduction of this historical package requires checking out the recorded Phase 00 commit. The current schema can be regenerated with `pnpm schema:snapshot`.

## Decision ledger

`retirement-ledger.json` is intentionally reviewed by humans. Every migration-era surface must be assigned exactly one final disposition:

- `KEEP` — current capability/invariant remains;
- `REPLACE` — a new canonical implementation takes ownership before the old one is removed;
- `DELETE` — no vNext runtime equivalent is required;
- `ARCHIVE-OFFLINE` — historical/evaluation value remains, but the artifact must leave serving runtime paths.

There is intentionally no `KEEP-JUST-IN-CASE` or `TBD` disposition.

## Root verification baseline

At the Phase 00 commit, the baseline branch used repository-local capture scripts plus `lint`, `typecheck`, `test`, and `build` before merge. Those one-time capture entrypoints were intentionally removed from the current tree in Phase 11; this directory is retained as immutable evidence rather than a live generator surface.

`pnpm validate:local-deploy` became the production-shaped integration gate for later behavior-changing phases.
