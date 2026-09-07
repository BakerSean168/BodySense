# BS-TECH-05 · deterministic repository tests with database invariants

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #5 · database CRUD unit/integration tests**

## Concept

deterministic repository tests with database invariants

## Prerequisites

- TECH-03

## BodySense target files

- `apps/api/internal/repository/body_state_repository_test.go`
- `apps/api/internal/repository/body_state_context_integration_test.go`
- `apps/api/internal/repository/body_state_repository.go`

## Prediction before reading/running

Pick one repository mutation and state what rows/fields should exist before and after, including one constraint/error case. Predict cleanup/isolation needs for repeated test runs.

## Task

Read one real repository test and classify whether it is a mock, unit, SQL-mock, or database integration test. Add or design one focused deterministic case that proves a persistence invariant rather than implementation call order.

## Failure case

Demonstrate how shared/random unbounded fixture state can make tests order-dependent or flaky. State how transaction cleanup or unique deterministic identities avoid it.

## Verification command / evidence

- `cd apps/api && go test ./internal/repository -run BodyState -count=1 -v`
- Repeat the focused test with `-count=10` if it is intended to be deterministic.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What behavior can a mock repository test not prove?
- Why does random test data not automatically mean good isolation?
- Which database constraint should be asserted by an integration test?

## Production change

adding a missing repository invariant test is encouraged; production repository code changes require a failing behavioral case.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
