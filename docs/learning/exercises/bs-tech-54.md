# BS-TECH-54 · durable background job lifecycle and queue/runtime ownership

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #54 · background worker with Redis/Asynq**

## Concept

durable background job lifecycle and queue/runtime ownership

## Prerequisites

- TECH-10
- TECH-25

## BodySense target files

- `apps/api/internal/service/job_runtime.go`
- `apps/api/internal/service/job_runtime_test.go`
- `apps/api/internal/repository/job_repository.go`

## Prediction before reading/running

Write the legal JobRuntime state transitions from memory, then predict what `ClaimPending` must do atomically to avoid two workers owning the same attempt.

## Task

Trace job creation, idempotency identity, claim, attempt count, transition validation, progress, terminal states, and event append. Compare BodySense DB-backed JobRuntime with a Redis queue such as Asynq without proposing a migration by default.

## Failure case

Model two workers claiming the same pending job and a worker crash after claim. State which invariant prevents duplicate ownership and which recovery rule makes stale running work recoverable.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "Job|Transition" -count=1 -v`
- `Inspect repository claim SQL/locking and, if untested, design a concurrent-claim integration test.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What is durable before a worker begins?
- What is the difference between idempotency key and retry attempt count?
- Why is appending an event after a status update a potential consistency topic?

## Production change

a missing concurrent-claim or recovery test is a valid improvement; do not replace the runtime with Asynq merely to mirror the source.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
