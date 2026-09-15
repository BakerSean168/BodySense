# BS-TECH-09 · PostgreSQL isolation plus application-level optimistic concurrency

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #9 · transaction isolation levels and read phenomena**

## Concept

PostgreSQL isolation plus application-level optimistic concurrency

## Prerequisites

- TECH-06
- TECH-07

## BodySense target files

- `apps/api/internal/database/transaction.go`
- `apps/api/internal/repository/body_state_repository.go`
- `apps/api/internal/repository/treatment_repository.go`

## Prediction before reading/running

Before running SQL, predict dirty-read, non-repeatable-read, phantom, and write-skew behavior under PostgreSQL READ COMMITTED and REPEATABLE READ. Then state which BodySense invariants rely on explicit revision/locking rather than isolation alone.

## Task

Use a disposable PostgreSQL database and two sessions to reproduce at least one read phenomenon at READ COMMITTED and compare with REPEATABLE READ. Map the result to BodyState revision checks and Treatment acceptance.

## Failure case

Create a two-session timeline where both actors read an old predicate/state and attempt incompatible writes. Explain whether PostgreSQL isolation, row locks, unique/check constraints, or BodySense expected revisions prevent it.

## Verification command / evidence

- A captured two-session SQL transcript or automated integration test
- `cd apps/api && go test ./internal/database ./internal/repository -count=1` after any test-only addition

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why does "PostgreSQL is ACID" not answer the isolation question?
- What guarantee does READ COMMITTED actually provide?
- Where does BodySense intentionally add stronger business concurrency control?

## Production change

no production isolation-level change without a demonstrated anomaly and a measured trade-off.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
