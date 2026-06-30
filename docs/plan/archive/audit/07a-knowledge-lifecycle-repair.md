# 07a Knowledge Lifecycle Schema Repair Report

## 1. What changed

Added migration `000020` to fix 5 gaps between the 07a plan and the `000019` implementation:

| Gap | Fix |
|-----|-----|
| `knowledge_units.lifecycle_status` missing | `ADD COLUMN lifecycle_status VARCHAR(50) NOT NULL DEFAULT 'generated'` |
| FK direction inverted (publications → units) | `ADD COLUMN publication_id UUID` on knowledge_units with FK → knowledge_publications |
| `knowledge_publications.status` defaults to `'published'` | `ALTER COLUMN status SET DEFAULT 'draft'` |
| `knowledge_publications` missing timestamps | `ADD COLUMN created_at/updated_at` + trigger |
| `license_status` too narrow (VARCHAR(30)) | `ALTER TYPE VARCHAR(50)` |

Updated `KnowledgePublication` Go model: added `CreatedAt`/`UpdatedAt`, changed Status default from `'published'` to `'draft'`.

## 2. Files changed

| File | Change |
|------|--------|
| `apps/api/migrations/000020_fix_knowledge_lifecycle_schema.up.sql` | New — corrective migration |
| `apps/api/migrations/000020_fix_knowledge_lifecycle_schema.down.sql` | New — rollback |
| `apps/api/internal/model/knowledge_publication.go` | Status default → `'draft'`, added `CreatedAt`/`UpdatedAt` |

## 3. Acceptance criteria result

| Criteria | Result | Evidence |
|----------|--------|----------|
| Knowledge lifecycle migration exists with up/down files | **PASS** | 000019 + 000020 both have up/down |
| `knowledge_publications` exists | **PASS** | 000019 creates it; 000020 adds timestamps and fixes default |
| `knowledge_units` has lifecycle, quality, publication, and content hash metadata | **PASS** | `lifecycle_status` (000020), `quality_score`+`content_hash` (000019), `publication_id` (000020) |
| `knowledge_sources` has license status metadata | **PASS** | 000019 adds it; 000020 widens to VARCHAR(50) |
| Existing search and ingestion behavior unchanged by default | **PASS** | All new columns have passive defaults; no query changes |

## 4. Verification

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./internal/model/...` | ✅ PASS | |
| `go build ./...` | ✅ PASS | |
| `go test ./... -count=1` | ✅ PASS (all packages) | 8 test packages, 0 failures |
| `uv run ruff check .` | ⚠️ 11 errors | Pre-existing, unrelated to this change |
| `uv run pytest tests/ -x -q` | ✅ PASS (177/177) | |

## 5. Remaining risks

- Migration `000020` assumes `update_updated_at_column()` function exists (created in earlier migrations). Verified by checking 000010.
- `publication_id` FK requires `knowledge_publications` table to exist first — 000019 creates it, 000020 runs after. Ordering is correct.
- Existing rows get `lifecycle_status = 'generated'` via default, which matches the plan's intent.

## 6. Next recommended blocker

**03b/03c ask_user E2E 链路修复** — Python `ask_user` not registered, no Python resume route, Go resume has no downstream effect.
