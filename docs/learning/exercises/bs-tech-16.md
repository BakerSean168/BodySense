# BS-TECH-16 · database error classification and public error translation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #16 · handle DB errors correctly**

## Concept

database error classification and public error translation

## Prerequisites

- TECH-11
- TECH-15

## BodySense target files

- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/repository/user_repository.go`
- `apps/api/internal/transport/httpapi/auth.go`
- `apps/api/internal/transport/httpapi/auth_test.go`

## Prediction before reading/running

Model two concurrent registrations for the same email. Both can pass a pre-check; predict the database error for the loser and what public response should be returned.

## Task

Trace errors from GORM/PostgreSQL through repository/service/handler for registration or another unique-constrained resource. Distinguish expected constraint conflict, not-found, transient DB failure, and internal bug.

## Failure case

Use the duplicate-registration race as the key failure case. Verify the public API does not return raw PostgreSQL/GORM details and does not misclassify a deterministic conflict as a generic 500.

## Verification command / evidence

- `cd apps/api && go test ./internal/handler ./internal/service -run "Auth|Registration" -count=1`
- `Add a focused repository/service test if the unique-constraint race is not characterized.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is `EmailExists` insufficient as the final uniqueness guarantee?
- Which layer should recognize a database constraint violation?
- Why should public error text differ from internal diagnostic detail?

## Production change

a proven misclassification or data leak justifies a production fix; otherwise document the existing boundary.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
