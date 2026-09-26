# BS-P8-CONCEPT-GRAPHQL-VARIABLES · named GraphQL operations plus variables separate query shape from dynamic runtime values

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-VARIABLES**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

named GraphQL operations plus variables separate query shape from dynamic runtime values.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY

## BodySense target files

- `apps/web/src/features/workspace/api/workspaceApi.ts`
- `apps/web/src/features/consultation/services`

## Prediction before reading/running

Write a named query or mutation with variables and predict which values are validated by GraphQL before resolver execution.

## Task

Use the Part 8 lab mutation to separate operation shape from runtime values. Compare GraphQL variables with REST path/query/body parameters and with TypeScript compile-time types.

## Failure case

Interpolate runtime values directly into GraphQL source strings or assume TypeScript typing removes the need for GraphQL/runtime validation.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and explain the variableized mutation.
- Change a local lab variable to an invalid enum/int shape, rerun the focused test manually, observe validation, then revert.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why use variables instead of string interpolation?
- Which layer validates variable shape?
- How is this different from TypeScript static typing?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
