# BS-P13-CONCEPT-FOREIGN-KEY-JOIN · one-to-many relations use a foreign key and joins/preloads to reconstruct related projections

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-FOREIGN-KEY-JOIN**

## Concept

one-to-many relations use a foreign key and joins/preloads to reconstruct related projections

## Prerequisites

- TECH-15

## BodySense target files

- `apps/api/migrations`
- `apps/api/internal/repository`

## Prediction before reading/running

Pick one relation such as messages -> conversations and predict FK column, parent key, delete/update behavior, owner filter and query join shape before reading the migration/repository.

## Task

Trace one BodySense FK relation from migration to repository join/preload. Explain referential integrity, delete/update policy, indexes and ownership filtering.

## Failure case

Join related rows without enforcing parent ownership or without an FK/index, allowing cross-user data leakage or slow/orphaned relations.

## Verification command / evidence

- Trace a concrete FK from a migration into `apps/api/internal/repository/message_context_repository.go` or consultation repositories.
- Run a focused repository test that asserts the join/owner predicate.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- What invariant does the FK enforce?
- Why does authorization still need an owner predicate?
- Which side needs an index and why?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
