# Phase 04b: Migrate OCR Processing to JobRuntime

## Goal

Replace the ad hoc OCR goroutine in `UploadService` with a narrow JobRuntime-backed OCR job while preserving current upload API behavior.

## Why

OCR is the smallest existing long task with clear input, progress, result, and failure states. Migrating it first validates JobRuntime without touching assessment, training, or knowledge ingestion.

## Current State

- `apps/api/internal/service/upload_service.go` validates and saves uploads.
- For `fileType == "report"`, it launches `go s.processOCR(upload.ID, userID, filePath, mimeType)`.
- `processOCR` updates `user_uploads.ocr_status` to `processing`, calls Python `/api/ocr/extract`, then writes `ocr_result` and final status.
- Failures are stored only on `user_uploads.ocr_result`.
- `apps/api/internal/model/user_upload.go` has `OCRStatus` and `OCRResult`.
- Frontend upload UI reads existing upload OCR fields.

## Scope

### Allowed

- Create an OCR job when a report upload is saved.
- Execute OCR through JobRuntime using a process-local worker/goroutine for this first version.
- Mirror job progress into existing `user_uploads.ocr_status` and `ocr_result` so frontend behavior remains unchanged.
- Add job events for OCR started/completed/failed.
- Add retry-safe idempotency around upload ID.

### Not Allowed

- Do not migrate assessment, training, reassessment, or knowledge ingestion.
- Do not change upload HTTP response shape except optionally adding `job_id` in metadata if non-breaking for current clients.
- Do not add frontend Job UI.
- Do not introduce external queue infrastructure.
- Do not remove `ocr_status` or `ocr_result` from `user_uploads`.

## Target Files

- `apps/api/internal/service/upload_service.go`
- `apps/api/internal/service/job_runtime.go` or `apps/api/internal/job/runtime.go`
- `apps/api/internal/repository/job_repository.go`
- `apps/api/internal/model/user_upload.go` (likely only if adding metadata/job id is necessary)
- `apps/api/internal/repository/upload_repository.go`
- `apps/api/internal/service/upload_service_test.go` (new/updated, likely)
- `apps/api/internal/service/ocr_job.go` (new, likely)

## Design Notes

Suggested job type:

```txt
upload.ocr_extract
```

Suggested job input:

```json
{
  "upload_id": "...",
  "file_path": "uploads/<user>/<file>",
  "mime_type": "application/pdf"
}
```

Behavior preservation:

- On upload create: `ocr_status = pending`.
- On job running: `ocr_status = processing`.
- On job completed: `ocr_status = completed`, `ocr_result = Python response`.
- On job failed: `ocr_status = failed`, `ocr_result = {"error": "..."}`

The worker can still be process-local in this ticket. The important shift is that the durable job row exists and records lifecycle events.

## Implementation Steps

1. Inject `JobRuntime` into `UploadService`.
2. Replace direct `go s.processOCR(...)` call with `JobRuntime.Enqueue` for `upload.ocr_extract`.
3. Add an OCR job handler that contains the current `processOCR` logic.
4. Move shared OCR HTTP call logic into a private method that receives context and job input.
5. When job starts, update both job status and `user_uploads.ocr_status`.
6. On success, complete job and update upload OCR result.
7. On failure, fail job and update upload OCR result with a structured error.
8. Use idempotency key such as `upload_ocr:<upload_id>`.
9. Add tests with a fake AI OCR server or fake OCR client where practical.
10. Confirm existing upload endpoints still return the same fields.

## Invariants

- Frontend upload list still works from `user_uploads.ocr_status`.
- Existing Python `/api/ocr/extract` endpoint remains unchanged.
- File validation and disk save behavior remain unchanged.
- Report uploads still trigger OCR automatically.
- Photo uploads do not create OCR jobs.

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

- [ ] Report upload creates a durable OCR job.
- [ ] OCR lifecycle writes job status and job events.
- [ ] Existing upload OCR fields are still updated.
- [ ] Duplicate enqueue for the same upload does not create duplicate active OCR work.
- [ ] No other long task is migrated.

## Regression Risks

- Process-local worker still loses in-flight work on restart; durable job row makes this visible but not fully recovered.
- UploadService constructor wiring changes may affect server startup.
- Idempotency may prevent legitimate manual retry if not modeled carefully.
- Existing frontend may ignore optional job metadata; preserve old fields.

## Out of Scope Follow-ups

- Generic job polling endpoint.
- Frontend job progress card.
- Retry scheduler.
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

