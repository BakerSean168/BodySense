# BS-P8-CONCEPT-APOLLO-CLIENT · Apollo Client combines GraphQL transport, normalized cache and React integration behind a provider/client instance

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P8-CONCEPT-APOLLO-CLIENT**
> Source course: **Full Stack Open Part 8 — GraphQL**

## Concept

Apollo Client combines GraphQL transport, normalized cache and React integration behind a provider/client instance.

## Prerequisites

- BS-P8-CONCEPT-GRAPHQL-SCHEMA-QUERY
- BS-P6-CONCEPT-TANSTACK-QUERY

## BodySense target files

- `apps/web/src/main.tsx`
- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`

## Prediction before reading/running

Predict the responsibilities of Apollo Client versus TanStack Query in BodySense: transport, query lifecycle, cache identity, React integration and invalidation/refetch policy.

## Task

Compare Apollo Client with the current BodySense QueryClient/TanStack Query setup without migrating production. Identify what GraphQL-aware normalization adds and what remains ordinary server-state ownership.

## Failure case

Assume Apollo Client is just an HTTP client, or assume installing it makes all React/application state server state.

## Verification command / evidence

- Run `pnpm curriculum:lab:graphql` and inspect the Apollo `InMemoryCache` portion of the lab.
- Trace `useHealthWorkspaceQuery.ts` and explain which responsibilities are equivalent and which differ.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer remains outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does Apollo Client know that a generic fetch wrapper does not?
- Which state should not be moved into Apollo/TanStack cache?
- Where does cache identity come from in each approach?

## Production change

Part 8 is an **isolated GraphQL learning/comparison track**. Do not migrate BodySense production from REST/SSE/TanStack Query merely to imitate the source course. Change production only when the exercise demonstrates a real defect or a separately justified architecture decision.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify it with the focused lab plus real BodySense code/test/runtime evidence, explain the failure mode, and independently justify the contract/ownership trade-off.
