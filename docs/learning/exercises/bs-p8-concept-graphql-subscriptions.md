# BS-P8-CONCEPT-GRAPHQL-SUBSCRIPTIONS · GraphQL subscriptions provide server-pushed operation results over a persistent transport

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-SUBSCRIPTIONS**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL subscriptions provide server-pushed operation results over a persistent transport.

## Prerequisites

- BS-P8-CONCEPT-APOLLO-CLIENT
- BS-P7-CONCEPT-SERVER-PUSH-SYNC

## BodySense target files

- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
- `apps/api/internal/stream`

## Prediction before reading/running

For a long-running BodySense Agent run, predict what GraphQL subscription/WebSocket transport changes compared with SSE and what it does not change about run identity, ordering, replay and cancellation.

## Task

Compare GraphQL subscriptions with the existing BodySense SSE runtime. Treat subscription as a push API abstraction and transport choice, not as durable execution authority.

## Failure case

Treat a WebSocket/subscription disconnect as business cancellation, or assume reconnect alone guarantees no lost/duplicated events.

## Verification command / evidence

- Trace `useSSEProcessor.ts` and `apps/api/internal/stream` after the existing server-push exercise.
- Explain how `(run_id, seq)`/replay requirements would remain if the transport changed to GraphQL subscriptions.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does a GraphQL subscription add over raw WebSocket?
- Which durable recovery invariants are transport-independent?
- When is SSE simpler than subscription/WebSocket?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
