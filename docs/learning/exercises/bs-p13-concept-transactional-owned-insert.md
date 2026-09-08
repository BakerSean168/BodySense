# BS-P13-CONCEPT-TRANSACTIONAL-OWNED-INSERT · authenticated creation should persist the resource with server-derived owner identity atomically with related invariants

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-TRANSACTIONAL-OWNED-INSERT**

## Concept

authenticated creation should persist the resource with server-derived owner identity atomically with related invariants

## Prerequisites

- TECH-06
- TECH-22

## BodySense target files

- `apps/api/internal/service`
- `apps/api/internal/repository`
- `apps/api/internal/database/transaction.go`

## Prediction before reading/running

For a user-owned write, predict how authenticated user identity enters the service, which rows must change atomically and what rollback state should remain after the second write fails.

## Task

Trace a BodySense user-owned write. Verify user ID comes from auth context, related updates share a transaction when necessary and a partial failure rolls back.

## Failure case

Trust a client-supplied owner ID or commit a parent row before a required related write fails, producing authorization or partial-state bugs.

## Verification command / evidence

- Trace a user-owned service/repository write and transaction helper in `apps/api/internal/database/transaction.go`.
- Run focused service/database tests that cover rollback/ownership or design the smallest characterization test if the selected path lacks one.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- Why must owner identity be server-derived?
- What makes two writes one atomic unit?
- Who decides transaction scope?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
