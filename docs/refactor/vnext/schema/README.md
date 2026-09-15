# BodySense vNext Canonical Database Schema

`database-schema.json` is the current PostgreSQL 18 schema snapshot produced from the active migration chain.

Regenerate it with the permanent schema tool:

```bash
pnpm schema:snapshot
```

The capture uses `pgvector/pgvector:pg18`, runs the repository migration validator through full up / latest down / replay up, and then records the resulting public tables and columns. The historical Phase 00 snapshot remains frozen under `docs/refactor/vnext/baseline/`.

The snapshot remains under the vNext evidence directory because it records the schema established by the engineering reset, but its generator is no longer refactor-only scaffolding.
