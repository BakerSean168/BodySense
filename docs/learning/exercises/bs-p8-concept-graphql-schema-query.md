# BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY · GraphQL schema defines a typed graph contract while client queries select exact response fields

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL schema defines a typed graph contract while client queries select exact response fields.

## Prerequisites

- None

## BodySense target files

- `apps/api/internal/handler`
- `packages/contracts/src`
- `apps/web/src/features/workspace/api/workspaceApi.ts`

## Prediction before reading/running

Take one BodySense REST response shape and predict an equivalent GraphQL schema plus a query that requests only two fields. Mark which fields may be null and which must not be.

## Task

Use the isolated Part 8 GraphQL lab and one real BodySense REST contract to compare endpoint-shaped JSON with a typed graph contract. Explain field selection, nullability, nested types and where runtime validation still belongs.

## Failure case

Assume a GraphQL schema proves that the underlying database/domain state is valid, or assume the server must return every field in a type even when the client did not select it.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and explain why the revision-only query contains no `facts` field.
- Compare the lab schema with one DTO in `packages/contracts/src` and identify one guarantee GraphQL adds and one guarantee it does not add.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What is the difference between a GraphQL type and a concrete query selection?
- What does `!` guarantee, and at which boundary?
- Why is GraphQL schema validation not a replacement for domain validation?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
