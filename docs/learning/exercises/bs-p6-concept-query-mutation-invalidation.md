# BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION · TanStack Query mutations synchronize remote changes through precise invalidation or cache updates

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P6-CONCEPT-QUERY-MUTATION-INVALIDATION**

## Concept

TanStack Query mutations synchronize remote changes through precise invalidation or cache updates.

## Prerequisites

- BS-P6-CONCEPT-TANSTACK-QUERY
- BS-FSO-2.17

## BodySense target files

- `apps/web/src/features/workspace/hooks/useBodyStateCommand.ts`
- `apps/web/src/features/workspace/hooks/useWorkspaceInvalidation.ts`

## Prediction before reading/running

Predict cache behavior after successful mutation, 409 stale revision and unrelated error. Which queries are invalidated and when is authoritative data fetched again?

## Task

Trace mutation success/error into BodySense invalidation. Explain key scope, concurrent mutation/stale revision handling and when setQueryData might be preferable.

## Failure case

Optimistically mutate a cache copy after a stale-revision conflict without reconciling server truth. Predict the inconsistency.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/workspace/hooks/useBodyStateCommand.test.tsx`
- Trace `useBodyStateCommand.ts` and `useWorkspaceInvalidation.ts` success/409 paths.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- invalidateQueries vs setQueryData: what correctness evidence is needed?
- Why refresh after 409?
- How broad should an invalidation key be?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
