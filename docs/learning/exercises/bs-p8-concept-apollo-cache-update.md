# BS-P8-CONCEPT-APOLLO-CACHE-UPDATE · after GraphQL mutation, client cache can be reconciled by refetching queries or targeted normalized cache writes

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-APOLLO-CACHE-UPDATE**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

after GraphQL mutation, client cache can be reconciled by refetching queries or targeted normalized cache writes.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-MUTATION
- BS-P8-CONCEPT-NORMALIZED-CACHE
- BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION

## BodySense target files

- `apps/web/src/features/workspace/hooks/useBodyStateCommand.ts`
- `apps/web/src/features/workspace/hooks/useWorkspaceInvalidation.ts`

## Prediction before reading/running

After a successful mutation, predict when a normalized cache update is sufficient and when a refetch/invalidation is safer because other server-derived data may have changed.

## Task

Compare Apollo cache writes/refetching with BodySense `setQueryData`/invalidation. Choose a reconciliation strategy for a stale-revision-sensitive body-state mutation and justify correctness before network cost.

## Failure case

Patch only the obvious object while leaving related derived queries stale, or refetch everything without understanding which server facts changed.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` for the normalized-cache baseline.
- Trace `useBodyStateCommand.ts` and `useWorkspaceInvalidation.ts`; identify what the current invalidation protects that a narrow manual cache patch could miss.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What is the correctness trade-off between targeted cache writes and refetch?
- Which server changes are not inferable from the mutation input?
- How would concurrent updates affect your choice?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
