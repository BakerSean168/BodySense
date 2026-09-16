# BS-TECH-15 · database-enforced identity and relationship constraints

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #15 · unique/foreign-key constraints**

## Concept

database-enforced identity and relationship constraints

## Prerequisites

- TECH-11

## BodySense target files

- `apps/api/migrations/000001_vnext_baseline.up.sql`
- `apps/api/migrations/000001_vnext_baseline.up.sql`
- `apps/api/internal/repository/user_repository.go`

## Prediction before reading/running

Predict which constraints enforce unique email and user-owned singleton/revision relationships, and what SQLSTATE/error class should occur on duplicates.

## Task

Trace at least one unique constraint and one FK/relationship constraint from migration DDL to repository behavior. Explain why pre-checks in application code cannot replace the database constraint under concurrency.

## Failure case

Construct two concurrent inserts that both pass an application-level existence check. Explain which database constraint chooses the loser and what the application must do with that error.

## Verification command / evidence

- A disposable PostgreSQL constraint experiment or existing repository integration test
- `cd apps/api && go test ./internal/repository -count=1` after any focused test addition.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why are uniqueness pre-checks race-prone?
- Which constraints protect integrity even if a service bug occurs?
- When would a check constraint be preferable to validation only in Go?

## Production change

only a proven missing integrity constraint justifies schema change; otherwise document and test existing constraints.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
