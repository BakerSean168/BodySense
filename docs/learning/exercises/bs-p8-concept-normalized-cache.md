# BS-P8-CONCEPT-NORMALIZED-CACHE · Apollo normalized cache identifies entities and reuses prior query results according to cache policy

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-NORMALIZED-CACHE**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

Apollo normalized cache identifies entities and reuses prior query results according to cache policy.

## Prerequisites

- BS-P8-CONCEPT-APOLLO-CLIENT
- BS-P6-CONCEPT-TANSTACK-QUERY

## BodySense target files

- `apps/web/src/features/workspace/hooks`
- `apps/web/src/features/consultation/services/consultationQueryOptions.ts`

## Prediction before reading/running

Predict how Apollo stores the same entity returned through two query paths when it has a stable typename/id, then contrast that with two TanStack query keys containing duplicate object data.

## Task

Use the Apollo cache lab to understand normalized entity identity. Compare entity normalization with BodySense query-key result caching and explain staleness/partial-entity trade-offs.

## Failure case

Assume normalization makes server data automatically fresh, or use unstable/missing entity identity and expect every projection to update correctly.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and explain why updating one normalized `BodyFact` changes the query projection.
- Compare the cache extract with a BodySense TanStack query key/result and describe the different identity models.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What key identifies a normalized entity?
- Does normalization solve freshness?
- When can partial selections make cache reasoning harder?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
