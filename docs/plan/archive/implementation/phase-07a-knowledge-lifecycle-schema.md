# Phase 07a: Knowledge Lifecycle Schema

## Goal

Extend the existing knowledge library schema with lifecycle, quality, and publication fields, without changing ingestion or retrieval behavior by default.

## Why

The current knowledge schema already has normalized sources, segments, units, and clips. The next step is not rebuilding it; it is adding lifecycle metadata so generated knowledge can be reviewed, published, deprecated, and safely filtered later.

## Current State

- `apps/api/migrations/000010_create_knowledge_library.up.sql` defines:
  - `knowledge_sources`
  - `knowledge_segments`
  - `knowledge_units`
  - `knowledge_clips`
- `knowledge_units.review_status` exists and defaults to `generated`.
- `knowledge_units.embedding VECTOR(1536)` exists.
- `knowledge_sources.ingest_status` exists.
- No `knowledge_publications` table exists.
- Python `apps/ai-service/src/rag/knowledge_library.py` performs search and ingestion against current schema.

## Scope

### Allowed

- Add migration to create `knowledge_publications`.
- Add lifecycle/quality/publication metadata columns to existing knowledge tables.
- Add Go/Python model or query type updates only where required for compilation.
- Add passive defaults that do not change current search results.
- Add tests or migration checks for default values where practical.

### Not Allowed

- Do not change `KnowledgeLibrary.search` default filtering yet.
- Do not migrate video ingestion to jobs.
- Do not implement admin review UI.
- Do not publish or deprecate existing knowledge in data.
- Do not rerun embeddings or bulk import Bilibili content.

## Target Files

- `apps/api/migrations/000019_add_knowledge_lifecycle.up.sql` (new, likely)
- `apps/api/migrations/000019_add_knowledge_lifecycle.down.sql` (new, likely)
- `apps/api/internal/model/knowledge_entry.go` or normalized knowledge models if present (likely)
- `apps/api/internal/handler/knowledge.go` (likely only if response structs need new fields)
- `apps/ai-service/src/rag/knowledge_library.py` (likely only if SQL column lists need explicit updates)
- `apps/ai-service/tests/unit/test_curated_source.py` or knowledge tests (likely)

## Design Notes

Additive columns:

```sql
knowledge_units.lifecycle_status VARCHAR(50) NOT NULL DEFAULT 'generated'
knowledge_units.quality_score DOUBLE PRECISION
knowledge_units.publication_id UUID REFERENCES knowledge_publications(id) ON DELETE SET NULL
knowledge_units.content_hash TEXT
knowledge_sources.license_status VARCHAR(50) NOT NULL DEFAULT 'unknown'
```

Publication table:

```sql
knowledge_publications (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  publication_key TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'draft',
  published_at TIMESTAMPTZ,
  created_by UUID,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)
```

Do not enforce published-only retrieval yet. That belongs to a later behavior ticket after data migration and evaluation.

## Implementation Steps

1. Check current latest migration number before adding lifecycle migration.
2. Add `knowledge_publications` table.
3. Add lifecycle columns to `knowledge_units`.
4. Add license status to `knowledge_sources`.
5. Add indexes for `knowledge_units.lifecycle_status`, `publication_id`, and quality filtering.
6. Add down migration that removes only these additions.
7. Update any explicit insert/select column lists that would fail after schema change.
8. Keep defaults compatible with existing generated data.
9. Add tests or SQL checks if the repo has migration validation.

## Invariants

- Existing knowledge search results remain unchanged.
- Existing ingestion continues to insert generated units successfully.
- Existing review_status behavior remains intact.
- No data backfill is required for MVP.
- No new UI or job runtime behavior is added.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
pnpm nx run ai-service:lint
pnpm nx run ai-service:test
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
cd ../ai-service
uv run ruff check .
uv run pytest tests/unit/test_curated_source.py tests/unit/test_video_pipeline.py
```

If a database migration validation command exists, run it and report the result.

## Acceptance Criteria

- [ ] Knowledge lifecycle migration exists with up/down files.
- [ ] `knowledge_publications` exists.
- [ ] `knowledge_units` has lifecycle, quality, publication, and content hash metadata.
- [ ] `knowledge_sources` has license status metadata.
- [ ] Existing search and ingestion behavior is unchanged by default.

## Regression Risks

- Migration defaults may unintentionally classify all existing units as publishable; use neutral `generated`.
- Existing raw SQL insert statements may omit required non-null fields if defaults are missing.
- UUID defaults require the same database extension assumptions as existing migrations.
- Later published-only filtering will need data review before enabling.

## Out of Scope Follow-ups

- Published-only retrieval filtering.
- Review/admin UI.
- Publication rollback behavior.
- Bilibili batch import.
- Knowledge ingestion jobs.

## Final Response Format for Coding Agent

```md
Changed files:
- ...

Behavior changes:
- ...

Tests run:
- ...

Tests passed / failed:
- ...

Known risks:
- ...

Follow-up tasks:
- ...
```

