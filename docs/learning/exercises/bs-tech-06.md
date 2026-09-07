# BS-TECH-06 · application-owned transaction boundary across multiple repositories

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #6 · clean database transaction**

## Concept

application-owned transaction boundary across multiple repositories

## Prerequisites

- TECH-03
- TECH-05

## BodySense target files

- `apps/api/internal/database/transaction.go`
- `apps/api/internal/database/transaction_test.go`
- `apps/api/internal/service/treatment_service.go`
- `apps/api/internal/repository/treatment_repository.go`
- `apps/api/internal/repository/body_state_repository.go`

## Prediction before reading/running

For `TreatmentService.RecordOutcome`, list every durable write that must commit together. Predict the database state if `RecordOutcome` projection fails after the outcome row is created.

## Task

Trace `TransactionManager.WithinTransaction` and `database.FromContext` through `TreatmentService.RecordOutcome`. Draw the transaction boundary and explain how repositories join the same `*gorm.DB` transaction through context.

## Failure case

Inject or model a failure after Outcome persistence but before BodyState projection/link update. Under the real transaction manager, prove that partial durable state is not committed.

## Verification command / evidence

- `cd apps/api && go test ./internal/database -run TransactionManager -count=1 -v`
- `cd apps/api && go test ./internal/service -run Treatment -count=1`
- `If no real-DB test proves the cross-repository rollback invariant, design/add one in a disposable PostgreSQL-backed test.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is `context.Context` carrying a transaction controversial but workable here?
- What would happen if one repository ignored `database.FromContext`?
- Where should the transaction begin: handler, service, or repository, and why for this flow?

## Production change

a missing rollback characterization test is a legitimate learning/quality improvement; do not rewrite transaction infrastructure without evidence.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
