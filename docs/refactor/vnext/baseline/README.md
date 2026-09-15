# BodySense vNext Phase 00 Baseline

This directory is the evidence package for `refactor/vnext-00-baseline`.

## Generated evidence

- `current-system-inventory.json` — public/internal routes, public stream variants, migration lineage, Agent manifests, referenced environment variables and broad code-quality discovery signals.
- `current-system-baseline.md` — human-readable summary generated from the inventory.
- `database-schema.json` — fresh PostgreSQL 18 + pgvector schema after applying migrations through version 62 and replaying the latest down/up migration.

`database-schema.json` is frozen Phase 00 evidence. Do not regenerate it from a later refactor phase: the current canonical schema has its own snapshot under `docs/refactor/vnext/schema/`. The inventory generator remains reproducible with `pnpm refactor:vnext:baseline`; exact Phase 00 schema reproduction requires checking out the recorded Phase 00 commit.

## Decision ledger

`retirement-ledger.json` is intentionally reviewed by humans. Every migration-era surface must be assigned exactly one final disposition:

- `KEEP` — current capability/invariant remains;
- `REPLACE` — a new canonical implementation takes ownership before the old one is removed;
- `DELETE` — no vNext runtime equivalent is required;
- `ARCHIVE-OFFLINE` — historical/evaluation value remains, but the artifact must leave serving runtime paths.

There is intentionally no `KEEP-JUST-IN-CASE` or `TBD` disposition.

## Root verification baseline

Phase 00 uses these repository-level checks before it can merge to the vNext integration branch:

```bash
pnpm refactor:vnext:baseline
pnpm refactor:vnext:schema
pnpm lint
pnpm typecheck
pnpm test
pnpm build
```

`pnpm validate:local-deploy` is the production-shaped integration gate for later behavior-changing phases. Phase 00 changes only planning/baseline tooling and therefore records rather than mutates runtime/deployment behavior.
