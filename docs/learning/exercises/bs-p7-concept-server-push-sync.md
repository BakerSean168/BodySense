# BS-P7-CONCEPT-SERVER-PUSH-SYNC · client/server synchronization may use polling, WebSockets, SSE or subscriptions depending on direction/reliability needs

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-SERVER-PUSH-SYNC**

## Concept

client/server synchronization may use polling, WebSockets, SSE or subscriptions depending on direction/reliability needs.

## Prerequisites

- BS-P6-CONCEPT-QUERY-MUTATION-INVALIDATION

## BodySense target files

- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
- `apps/api/internal/stream`

## Prediction before reading/running

Compare polling, WebSocket, GraphQL subscription and SSE for a one-way streamed Agent run. Predict reconnect/replay requirements after receiving events 1..N and disconnecting.

## Task

Compare polling/WebSocket/GraphQL subscriptions with BodySense SSE consultation stream. Explain directionality, reconnect/replay, ordering and durable recovery concerns.

## Failure case

Treat transport disconnect as business cancellation or apply replayed events twice. Predict user-visible corruption.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/hooks/useSSEProcessor.test.ts apps/web/src/features/consultation/runtime/durableRunRecovery.test.ts`
- Trace server stream/public event persistence and browser deduplication/recovery.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why choose SSE for a one-way stream?
- What makes reconnect safe?
- How is transport liveness separated from durable run state?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
