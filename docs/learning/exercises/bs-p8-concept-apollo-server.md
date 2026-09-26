# BS-P8-CONCEPT-APOLLO-SERVER · Apollo Server binds GraphQL schema plus resolver implementation into an HTTP GraphQL runtime

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-APOLLO-SERVER**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

Apollo Server binds GraphQL schema plus resolver implementation into an executable GraphQL server runtime.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY

## BodySense target files

- `scripts/learning/labs/part8-graphql-lab.mjs`
- `apps/api/cmd/server/main.go`
- `apps/api/internal/handler`

## Prediction before reading/running

Before opening the lab, predict what Apollo Server must own versus what the schema, resolver and BodySense domain service should own. Identify where request execution starts and where business authority must remain.

## Task

Trace the isolated Apollo Server lab from `typeDefs` through resolver execution, then compare that composition with BodySense Gin route/handler/service wiring. Explain why introducing GraphQL changes the API adapter but does not move domain authority into Apollo.

## Failure case

Put persistence/authorization/domain policy directly into schema declarations or treat Apollo Server as the business-service layer rather than an API runtime.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and identify the Apollo Server `executeOperation` path used by the tests.
- Compare the lab resolver composition with `apps/api/cmd/server/main.go` plus one current handler/service path.

Passing an existing test is **not** sufficient for L4. The learner must explain which boundary the test proves and what observation would falsify their ownership model.

## Explain-back questions

- What does Apollo Server add on top of GraphQL schema semantics?
- Which responsibilities belong to resolvers versus domain services?
- If BodySense exposed GraphQL tomorrow, which existing layers should remain unchanged?

## Production change

Part 8 uses an isolated Apollo/GraphQL lab. Do not migrate BodySense production from Gin REST/SSE merely to imitate the source course.

## L4 acceptance

Complete only when the learner can predict the server execution path, verify it in the focused lab, map it onto the real BodySense adapter/service boundary, and explain a failure mode that would violate that separation.
