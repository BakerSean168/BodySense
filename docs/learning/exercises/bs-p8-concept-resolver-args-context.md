# BS-P8-CONCEPT-RESOLVER-ARGS-CONTEXT · GraphQL resolvers receive parent, arguments, shared context and execution info with distinct responsibilities

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-RESOLVER-ARGS-CONTEXT**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL resolvers receive parent, arguments, shared context and execution info with distinct responsibilities.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY

## BodySense target files

- `apps/api/internal/handler`
- `apps/api/internal/service`

## Prediction before reading/running

For a nested GraphQL field, predict which values belong to `parent`, `args`, request `context` and execution `info`. Then map each role to the closest BodySense handler/service concept.

## Task

Trace a BodySense handler into its service and compare that boundary with GraphQL resolver execution. Separate client-controlled arguments from server-controlled authenticated/request context and parent-field data.

## Failure case

Trust a user id or authorization fact from GraphQL arguments when it should come from authenticated server context, or put business persistence directly in every field resolver.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` to establish the schema/resolver baseline.
- Trace one `apps/api/internal/handler` request into `apps/api/internal/service` and identify what would be `args`, `context` and service-owned state in a GraphQL adapter.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Which resolver inputs are client-controlled?
- What belongs in request context rather than arguments?
- Why should resolvers stay thin around domain services?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
