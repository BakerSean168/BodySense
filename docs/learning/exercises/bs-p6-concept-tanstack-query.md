# BS-P6-CONCEPT-TANSTACK-QUERY · TanStack Query owns remote cache, query lifecycle, deduplication and refetch policy

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P6-CONCEPT-TANSTACK-QUERY**

## Concept

TanStack Query owns remote cache, query lifecycle, deduplication and refetch policy.

## Prerequisites

- BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE
- BS-FSO-2.11

## BodySense target files

- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`
- `apps/web/src/features/consultation/services/consultationQueryOptions.ts`

## Prediction before reading/running

For one query, predict cache identity, first-load status, stale/fetching differences, deduplication and what a second component using the same key observes.

## Task

Trace a BodySense query key/queryFn/result into a component. Explain cache identity, stale/fetching status and why this is server-state rather than global client state.

## Failure case

Use two different resources under the same query key or omit identity fields. Predict cache aliasing and incorrect UI.

## Verification command / evidence

- Trace `useHealthWorkspaceQuery.ts` or consultation query options from query key -> queryFn -> component.
- Use existing focused web tests or devtools/code trace to explain stale versus fetching versus cached data.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes two queries the same cache entry?
- Why is `isFetching` different from “no data yet”?
- Who decides when cached data is stale?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
