# BS-P8-CONCEPT-GRAPHQL-MUTATION · GraphQL mutations model state-changing operations separately from queries but still require domain authority and durable semantics

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-MUTATION**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL mutations model state-changing operations separately from queries but still require domain authority and durable semantics.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY
- BS-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY

## BodySense target files

- `apps/api/internal/handler`
- `apps/api/internal/service`
- `apps/web/src/features/workspace/hooks/useBodyStateCommand.ts`

## Prediction before reading/running

Translate one BodySense POST/PATCH command into a GraphQL mutation. Predict how `expectedRevision`, authentication and the authoritative returned state should behave on success.

## Task

Use the lab mutation and `useBodyStateCommand.ts` to compare REST command semantics with GraphQL mutation semantics. Keep stale-write protection and domain authority independent of transport style.

## Failure case

Treat the word `mutation` as permission to perform arbitrary side effects, or update the client optimistically without preserving the server revision invariant.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and explain the successful mutation test.
- Trace the real BodySense body-state command and identify the invariant that must survive a hypothetical GraphQL adapter.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- How is a GraphQL mutation different from an HTTP method?
- Where should stale-revision enforcement live?
- Why should the mutation return authoritative server state?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
