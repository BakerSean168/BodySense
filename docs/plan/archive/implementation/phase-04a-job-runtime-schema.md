# Phase 04a: JobRuntime Schema and Go Module Skeleton

## Goal

Add the Go `jobs` and `job_events` schema plus a minimal `JobRuntime` Module, without migrating any existing long task yet.

## Why

OCR, reassessment, assessment, training plan generation, and knowledge ingestion are long-running or retryable tasks. Before migrating behavior, the system needs a stable job state model and Module interface.

## Current State

- There is no `jobs` or `job_events` table.
- `UploadService.UploadFile` launches OCR with `go s.processOCR(...)`.
- `TrainingService` and assessment flows call AI services synchronously in their own Modules.
- Frontend profile upload UI reads `user_uploads.ocr_status` directly.
- No generic job progress event contract exists.

## Scope

### Allowed

- Add migrations for `jobs` and `job_events`.
- Add Go models, repository, and `JobRuntime` skeleton.
- Define job statuses, types, progress payload, result payload, and error payload.
- Add unit tests for status transitions.
- Add contracts for job stream events if needed, but do not emit them from existing flows yet.

### Not Allowed

- Do not migrate OCR in this ticket.
- Do not add workers that execute real tasks.
- Do not change upload, assessment, training, or knowledge ingestion behavior.
- Do not add frontend Job UI.
- Do not introduce external queue/Temporal/Redis queue.

## Target Files

- `apps/api/migrations/000017_create_jobs.up.sql` (new, likely)
- `apps/api/migrations/000017_create_jobs.down.sql` (new, likely)
- `apps/api/internal/model/job.go` (new, likely)
- `apps/api/internal/model/job_event.go` (new, likely)
- `apps/api/internal/repository/job_repository.go` (new, likely)
- `apps/api/internal/service/job_runtime.go` or `apps/api/internal/job/runtime.go` (new, likely)
- `apps/api/internal/service/job_runtime_test.go` or package test (new, likely)
- `packages/contracts/src/stream-events.ts` (likely, only for job event type additions)
- `packages/contracts/schemas/stream-event.v1.schema.json` (likely)

## Design Notes

Suggested job statuses:

```txt
pending
running
waiting_user
completed
failed
cancelled
timed_out
```

Suggested job fields:

```sql
jobs (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  user_id UUID NOT NULL,
  conversation_id UUID,
  run_id UUID,
  job_type VARCHAR(100) NOT NULL,
  status VARCHAR(30) NOT NULL,
  input JSONB NOT NULL DEFAULT '{}',
  progress JSONB NOT NULL DEFAULT '{}',
  result JSONB,
  error JSONB,
  idempotency_key TEXT,
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  metadata JSONB NOT NULL DEFAULT '{}'
)
```

`job_events` should be append-only and include `event_type`, `payload`, and timestamps.

## Implementation Steps

1. Check current latest migration number before adding new migration files.
2. Add `jobs` and `job_events` migrations with indexes for user/status/type and idempotency.
3. Add Go models for both tables.
4. Add repository methods:
   - `Create`
   - `GetByIDForUser`
   - `MarkRunning`
   - `UpdateProgress`
   - `Complete`
   - `Fail`
   - `Cancel`
   - `AppendEvent`
5. Add `JobRuntime` service wrapping repository transitions.
6. Enforce legal state transitions in the runtime, not in handlers.
7. Add unit tests for legal and illegal transitions.
8. If adding contract types, add only passive TypeScript definitions for `job.created`, `job.progress`, `job.completed`, and `job.failed`.

## Invariants

- Existing upload/OCR behavior remains unchanged.
- No new background worker executes tasks.
- Job state transitions are centralized in `JobRuntime`.
- `job_events` are append-only.
- Migration down file reverses only this ticket's tables.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
pnpm nx run contracts:typecheck
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
```

## Acceptance Criteria

- [ ] `jobs` and `job_events` migrations exist.
- [ ] Go models/repository/runtime skeleton exist.
- [ ] Legal job transition tests pass.
- [ ] No existing flow is migrated to jobs.
- [ ] No external queue dependency is introduced.

## Regression Risks

- Migration numbering conflicts with concurrent schema work.
- Overly rigid transition rules may not fit OCR migration; keep extensible metadata.
- Missing indexes could make future job list endpoints slow.
- Contract updates may fail if contracts package lacks a typecheck target; report clearly.

## Out of Scope Follow-ups

- OCR migration.
- Job worker polling.
- Job progress UI.
- Retry scheduler.

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

