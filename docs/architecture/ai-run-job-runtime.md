# AI Run / Job Runtime

- Status: Current implemented runtime with explicit remaining scope
- Updated: 2026-09-01
- Historical design snapshot: [`docs/plan/archive/architecture-snapshots/2026-09-01/ai-run-job-runtime.md`](../plan/archive/architecture-snapshots/2026-09-01/ai-run-job-runtime.md)

## 1. Purpose

`JobRuntime` is the Go-owned durable lifecycle for background work that must survive request termination and support idempotency/recovery.

```text
HTTP/application command
  -> create durable Job
  -> claim/transition
  -> execute bounded work
  -> persist progress/result/error
  -> recover pending/stale jobs after restart
```

A Job is not the same thing as a conversational Agent Run:

```text
Run
  = checkpointed/streaming Agent execution identity

Job
  = durable background-work lifecycle
```

## 2. Current implementation

| Capability                           | Status                             | Current evidence                                                                                   |
| ------------------------------------ | ---------------------------------- | -------------------------------------------------------------------------------------------------- |
| `jobs` / `job_events` durable schema | ✅                                 | migrations + `JobRepository`                                                                       |
| Job state machine                    | ✅                                 | pending/running/waiting_user/completed/failed/cancelled/timed_out                                  |
| Idempotent job creation              | ✅                                 | `CreateJobWithIdempotency*`                                                                        |
| Claim/recovery primitives            | ✅                                 | `ClaimPending`, `ListRecoverable`                                                                  |
| Progress/result/error persistence    | ✅                                 | `UpdateProgress`, `TransitionTo`                                                                   |
| Report OCR                           | ✅                                 | `upload_ocr:<upload_id>` JobRuntime path                                                           |
| Posture analysis                     | ✅                                 | upload posture JobRuntime path with recovery/timeout handling                                      |
| Knowledge ingestion                  | ✅ partial/current                 | durable knowledge ingestion job integration exists; deeper pipeline workers remain domain-specific |
| Assessment generation                | synchronous                        | intentionally remains request/application operation today                                          |
| Treatment/training generation        | mixed synchronous/domain workflows | not universally migrated to JobRuntime                                                             |
| Title generation                     | utility path, not JobRuntime       | cosmetic role; no requirement to force every LLM call into a Job                                   |
| Generic Job SSE UI                   | open                               | progress is durable but no universal user-facing Job stream is required yet                        |
| Separate Python JobWorker authority  | not a current target               | Go remains job truth; Python is invoked as bounded computation where needed                        |

## 3. OCR path

The historical request-goroutine OCR design is retired.

Current flow:

```text
UploadService.Upload
  -> enqueueOCRJob
     idempotency_key = upload_ocr:<upload_id>
  -> JobRuntime durable pending job
  -> processOCRJob
  -> running + progress
  -> read upload through UploadStorage
  -> call Python OCR mechanism
  -> persist OCR result/status
  -> completed / failed / timed_out
```

On startup/recovery, `UploadService` scans recoverable OCR jobs. A service restart therefore does not silently lose OCR work merely because a request goroutine disappeared.

## 4. Posture path

Posture analysis also uses JobRuntime rather than a request-owned background goroutine:

```text
upload photo
  -> durable posture job
  -> Posture Agent/perception execution
  -> validate result/config identity
  -> user_uploads.analysis_result
  -> complete job
```

Completed governed Posture output may later become Assessment evidence. Assessment itself never reopens raw images for visual inference.

## 5. Job state and idempotency

Canonical lifecycle:

```text
pending
  -> running
     -> completed
     -> failed
     -> timed_out
     -> waiting_user   # where a job type supports it
  -> cancelled
```

Job types should use a stable business idempotency key when duplicate enqueue requests represent the same work. Upload jobs use the durable upload ID rather than request identity.

## 6. Recovery semantics

Recovery is based on durable job state, not in-memory goroutine presence.

```text
startup/reconciler
  -> ListRecoverable(job_type, stale_after, limit)
  -> claim/retry/timeout according to job-type policy
```

A job processor must make its persistence path idempotent enough that a retry does not create a second business artifact accidentally.

## 7. Why not every AI call is a Job

The historical design suggested progressively migrating Assessment, Training and Title merely because they call AI. That is no longer a universal architectural goal.

Use JobRuntime when the business operation needs durable background-work semantics:

- request may end before work completes;
- restart recovery matters;
- retries/idempotency need a durable identity;
- progress is user/business relevant;
- processing can be decoupled from an immediate response.

Do **not** force a low-latency synchronous role or checkpointed Agent Run through JobRuntime just for platform uniformity.

## 8. Ownership

```text
Go JobRuntime
  -> job identity / state / idempotency / recovery / durable result envelope

Python
  -> OCR/Posture/LLM/RAG computation invoked by the owning application/job path
  -> does not become durable Job authority
```

This matches the wider principle that mechanisms/Agents can execute in Python while Go owns durable application truth.

## 9. Remaining work

Open items must be justified by a concrete product/runtime need rather than an old phase checklist:

- generic Job progress projection/UI if multiple user-facing background tasks need it;
- stronger per-job retry/backoff policies where transient providers justify retries;
- additional job-type recovery tests as new background operations are added;
- optional worker separation only if current in-process execution becomes an availability/scaling bottleneck.

These are tracked as implementation gaps only when accepted by an active plan. The current documentation/code alignment audit is [`2026-09-01-documentation-code-alignment-audit.md`](../plan/active/2026-09-01-documentation-code-alignment-audit.md).
