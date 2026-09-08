# BS-P2-CONCEPT-ASYNC-RUNTIME · asynchronous browser runtime and non-blocking I/O

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P2-CONCEPT-ASYNC-RUNTIME**

## Concept

asynchronous browser runtime and non-blocking I/O

## Prerequisites

- BS-FSO-0.5

## BodySense target files

- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`
- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`

## Prediction before reading/running

Trace a browser network operation and predict what JavaScript continues doing while I/O is pending and when the completion callback/microtask can run.

## Task

Predict the ordering of synchronous render code, network completion, promise callbacks, and React state updates; verify with a controlled trace.

## Failure case

Block the main thread with synchronous work and assume network completion can update UI during that block.

## Verification command / evidence

- Trace an API request in `apps/web/src/lib/api-client.ts` and an SSE callback in `useSSEProcessor.ts`.
- Run `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/hooks/useSSEProcessor.test.ts`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does non-blocking I/O actually mean?
- Who owns the network operation while JS continues?
- What still blocks the browser main thread?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
