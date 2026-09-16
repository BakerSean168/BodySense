# BS-P8-CONCEPT-GRAPHQL-AUTH-CONTEXT · GraphQL authentication commonly resolves the current user into request context for resolver authorization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-AUTH-CONTEXT**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

GraphQL authentication commonly resolves the current user into request context for resolver authorization.

## Prerequisites

- BS-P8-CONCEPT-RESOLVER-ARGS-CONTEXT
- BS-P4-CONCEPT-BEARER-AUTHORIZATION

## BodySense target files

- `apps/api/internal/middleware/auth.go`
- `apps/api/internal/handler`
- `apps/api/internal/service`

## Prediction before reading/running

Predict where a bearer token should be validated and how the authenticated user should become resolver context. Then predict which resource-level authorization checks must still happen after authentication.

## Task

Compare Apollo-style request context with BodySense auth middleware and service authorization. Keep authentication identity server-controlled and separate from resource ownership/domain permission.

## Failure case

Accept `userId` from GraphQL arguments as proof of identity, or authenticate once and skip object-level authorization in resolvers/services.

## Verification command / evidence

- Trace `apps/api/internal/middleware/auth.go` into one authenticated handler/service path.
- Explain how the same verified identity would be injected into GraphQL context without moving authorization authority into the client.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Authentication versus authorization?
- Why is resolver context server-controlled?
- Where should object-level ownership checks live?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
