# BS-P8-CONCEPT-GRAPHQL-ERRORS · GraphQL validates schema/operation shape automatically while domain/business errors still need explicit structured handling

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-ERRORS**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL validates schema/operation shape automatically while domain/business errors still need explicit structured handling.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-MUTATION
- BS-P3-CONCEPT-HTTP-ERROR-TAXONOMY

## BodySense target files

- `apps/api/internal/handler/utils.go`
- `apps/web/src/lib/api-client.ts`

## Prediction before reading/running

Predict the observable difference between an invalid GraphQL operation and a valid mutation that violates a BodySense domain invariant such as stale revision.

## Task

Separate parse/schema/operation validation from resolver/domain failures. Compare GraphQL error payload semantics with BodySense REST HTTP status/error-model semantics.

## Failure case

Treat HTTP 200 as proof that a GraphQL operation succeeded, or collapse schema validation and domain conflict errors into one undifferentiated message.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and inspect the stale-revision GraphQL error extensions.
- Compare with BodySense HTTP error handling and explain how a client must inspect GraphQL execution errors even when transport succeeds.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Which failures happen before resolver execution?
- Why can a GraphQL response carry errors over a successful HTTP transport?
- How would you preserve a stable domain error code?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
