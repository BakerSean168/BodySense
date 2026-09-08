# BS-P13-CONCEPT-EAGER-VS-LAZY-LOAD · eager loading fetches related data with the primary query while lazy loading defers relation queries until needed

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P13-CONCEPT-EAGER-VS-LAZY-LOAD**

## Concept

eager loading fetches related data with the primary query while lazy loading defers relation queries until needed

## Prerequisites

- BS-P13-CONCEPT-RELATIONAL-QUERYING

## BodySense target files

- `apps/api/internal/repository`
- `apps/api/internal/service/health_workspace_service.go`

## Prediction before reading/running

For a read model with parent/child data, predict query count and payload shape under one joined/eager query versus deferred per-parent loads.

## Task

Find a BodySense preload/join or multi-query projection. Compare eager versus lazy cost, N+1 risk, payload overfetch and request-lifetime consistency.

## Failure case

Lazy-load children in a loop and create N+1 behavior, or eagerly fetch large unused relations on every request.

## Verification command / evidence

- Inspect BodySense join/multi-query projections in repositories and `health_workspace_service.go`.
- Create a query-count/payload reasoning table and identify what logging/benchmark would distinguish the alternatives.

Passing an existing test is **not** sufficient for L4. The learner must explain the invariant, the untested boundary and a falsifying observation.

## Explain-back questions

- What are eager and lazy loading?
- How does N+1 arise?
- When can multiple explicit queries be better than one giant join?

## Production change

No production change is required when the current persistence design already satisfies the concept. If a real defect or observability gap is found, first add a focused characterization/regression test, then make the smallest architecture-consistent correction.

## L4 acceptance

Complete only when the learner can predict the schema/query/transaction behavior before reading the answer path, verify it with code/test/SQL evidence, explain the failure mode and identify what would falsify the conclusion.
