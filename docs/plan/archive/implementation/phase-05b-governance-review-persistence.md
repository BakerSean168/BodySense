# Phase 05b: Governance Review Persistence

## Goal

Persist AI output governance results in Go through an `ai_output_reviews` table and service, without fully enforcing all outputs yet.

## Why

After Python can produce a structured governance result, Go needs an audit trail before business tables are written. This ticket records governance outcomes so bad cases can be reviewed and later added to evaluation sets.

## Current State

- Phase 05a should provide Python `GovernanceResult`.
- Go services such as `AssessmentService.GenerateAssessment` and `TrainingService.GeneratePlan` call Python AI endpoints and then write business tables directly.
- There is no `ai_output_reviews` table.
- No Go output review service exists.
- Current AI client methods return JSON but do not expose review metadata.

## Scope

### Allowed

- Add `ai_output_reviews` migration, model, repository, and service.
- Define Go DTO for governance result payloads returned from Python.
- Persist review records associated with `run_id`, `job_id`, `conversation_id`, or business object IDs when available.
- Integrate one narrow path for persistence in observe-only mode if Python endpoint already returns governance metadata.
- Add tests for review persistence and status mapping.

### Not Allowed

- Do not block business writes based on review status in this ticket unless the path is explicitly designed as observe-only failure safe.
- Do not rewrite all AI endpoints.
- Do not add UI for review browsing.
- Do not add LLM repair or retry.
- Do not migrate knowledge lifecycle review; that belongs to Phase 07a.

## Target Files

- `apps/api/migrations/000018_create_ai_output_reviews.up.sql` (new, likely)
- `apps/api/migrations/000018_create_ai_output_reviews.down.sql` (new, likely)
- `apps/api/internal/model/ai_output_review.go` (new, likely)
- `apps/api/internal/repository/ai_output_review_repository.go` (new, likely)
- `apps/api/internal/service/output_review_service.go` (new, likely)
- `apps/api/internal/service/ai_client.go` (likely, DTO additions)
- `apps/api/internal/service/assessment_service.go` (likely, optional observe-only integration)
- `apps/api/internal/service/training_service.go` (likely, optional observe-only integration)

## Design Notes

Suggested table:

```sql
ai_output_reviews (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id UUID NOT NULL,
  run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
  job_id UUID REFERENCES jobs(id) ON DELETE SET NULL,
  conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
  output_type VARCHAR(100) NOT NULL,
  status VARCHAR(30) NOT NULL,
  raw_output JSONB,
  validated_output JSONB,
  issues JSONB NOT NULL DEFAULT '[]',
  prompt_version TEXT,
  model TEXT,
  provider TEXT,
  business_ref_type VARCHAR(100),
  business_ref_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  metadata JSONB NOT NULL DEFAULT '{}'
)
```

Observe-only mode:

- Persist the review if present.
- If review persistence fails, log and continue for the first integration.
- Do not change final user result in this ticket.

## Implementation Steps

1. Add migration and model for `ai_output_reviews`.
2. Add repository methods `Create` and `ListByBusinessRef` if useful.
3. Add `OutputReviewService.Record(...)`.
4. Add Go DTO matching Python `GovernanceResult`.
5. Update `AIClient` response decoding only if Python endpoints include governance metadata.
6. Integrate one path in observe-only mode, preferably assessment or treatment, based on the least invasive response shape.
7. Add tests for repository/service creation and JSON fields.
8. Document in code comments that enforcement is a later ticket.

## Invariants

- Business writes continue as before.
- Governance persistence failure does not break user flow in observe-only mode.
- Raw output storage must avoid secrets and full prompt unless explicitly approved.
- Review statuses match Python guard statuses.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
```

## Acceptance Criteria

- [ ] `ai_output_reviews` migration/model/repository/service exists.
- [ ] Go can persist a governance result with issues and validated output.
- [ ] At least one observe-only integration path is implemented or clearly deferred if Python endpoints do not expose review metadata yet.
- [ ] No blocking enforcement is added.
- [ ] No review UI is added.

## Regression Risks

- Storing large raw outputs may grow DB quickly; keep payload bounded or document limits.
- Linking reviews to business refs may be ambiguous before jobs/runs cover all AI calls.
- Python and Go status enums may drift if not tested through fixtures.
- Migration numbering may conflict.

## Out of Scope Follow-ups

- Enforcement gates.
- Review dashboard.
- Bad-case export pipeline.
- LLM repair/retry.

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

