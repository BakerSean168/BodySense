# BS-FSO-0.6 · SPA mutation -> cache invalidation -> new projection

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-0.6 · New note in Single page app diagram**

## Concept

SPA mutation -> cache invalidation -> new projection

## Prerequisites

- FSO-0.4
- FSO-0.5

## BodySense target files

- `apps/web/src/features/workspace/hooks/useBodyStateCommand.ts`
- `apps/web/src/features/workspace/hooks/useWorkspaceInvalidation.ts`
- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`
- `apps/web/src/features/workspace/api/workspaceApi.ts`

## Prediction before reading/running

Predict what React state/cache changes immediately after a successful BodyState command and after a 409 conflict. State explicitly what is not locally mutated.

## Task

Trace `useBodyStateCommand` from mutation invocation through `onSuccess`/`onError` and query invalidation. Draw the SPA-side sequence that causes the durable server result to become the next rendered projection.

## Failure case

Assume the server commits successfully but workspace invalidation is removed. Predict the visible stale-state failure and how a reload differs from the in-session UI.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/workspace/hooks/useBodyStateCommand.test.tsx`
- Inspect the query keys invalidated by `useWorkspaceInvalidation` and identify the next network read.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is TanStack Query data not copied into Zustand?
- Why does a 409 trigger invalidation even though the mutation failed?
- What is the difference between optimistic UI and authoritative server reconciliation here?

## Production change

none unless a cache-reconciliation failure is demonstrated by a test.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
