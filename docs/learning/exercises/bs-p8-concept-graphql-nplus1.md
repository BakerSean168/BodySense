# BS-P8-CONCEPT-GRAPHQL-NPLUS1 · field resolvers can create N+1 database queries; batching/join/preload strategies restore bounded query cost

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-GRAPHQL-NPLUS1**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

field resolvers can create N+1 database queries; batching/join/preload strategies restore bounded query cost.

## Prerequisites

- BS-P8-CONCEPT-RESOLVER-ARGS-CONTEXT
- BS-P13-CONCEPT-RELATIONAL-QUERYING

## BodySense target files

- `apps/api/internal/repository`
- `apps/api/internal/service/health_workspace_service.go`

## Prediction before reading/running

For a query returning N parent objects with one nested relation each, predict the database query count for naive per-field loading versus join/preload/batching.

## Task

Construct the GraphQL N+1 mental model and compare it with BodySense GORM/repository query behavior. Explain why a convenient field resolver can hide unbounded database work.

## Failure case

Measure only GraphQL response correctness and miss N+1 database amplification, or assume ORM/preload behavior is efficient without query-count evidence.

## Verification command / evidence

- Inspect one BodySense repository/service relation query and state the expected SQL/query count.
- Use query logs/SQL mock/EXPLAIN reasoning to describe an observation that would falsify your N+1 prediction.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why does N+1 arise naturally in field resolvers?
- How do join, preload and batching differ?
- What metric/log evidence reveals the problem?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
