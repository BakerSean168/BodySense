# BS-FSO-2.11 · initial server fetch and trusted client projection

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-2.11 · initial data from server**

## Concept

initial server fetch and trusted client projection

## Prerequisites

- FSO-0.5

## BodySense target files

- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`
- `apps/web/src/features/workspace/api/workspaceQueryOptions.ts`
- `apps/web/src/features/workspace/api/workspaceApi.ts`
- `apps/web/src/lib/api-client.ts`

## Prediction before reading/running

Predict the exact function chain from `useHealthWorkspaceQuery` to `fetch`, when the returned Promise resolves, and where an HTTP error becomes an exception instead of data.

## Task

Trace the initial HealthWorkspace query end to end on the web side. Explain the roles of query key, query function, API wrapper, `expectJson`, and the resulting cache entry.

## Failure case

Model a 401, a 500/non-JSON body, and a successful response with stale domain content. Explain which failures the client can detect at transport level and which require server/domain contracts.

## Verification command / evidence

- `Focused Vitest on workspace query/API helpers if present; otherwise add a test only if an observable contract is missing`
- One controlled Network trace can serve as runtime evidence.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What does TanStack Query add beyond `useEffect + useState`?
- Why does HTTP 200 not prove domain freshness?
- Where should runtime JSON validation live if a boundary needs it?

## Production change

none unless the API/query error boundary lacks a needed contract or test.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
