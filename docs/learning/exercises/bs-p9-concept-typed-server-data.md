# BS-P9-CONCEPT-TYPED-SERVER-DATA · typing an HTTP client response does not validate the network payload; runtime parsing is required at trust boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-TYPED-SERVER-DATA**

## Concept

typing an HTTP client response does not validate the network payload; runtime parsing is required at trust boundaries.

## Prerequisites

- BS-P9-CONCEPT-TYPE-ERASURE
- BS-P9-CONCEPT-UNKNOWN-NARROWING
- BS-FSO-2.11

## BodySense target files

- `apps/web/src/lib/api-client.ts`
- `packages/contracts/src/stream-event-parser.ts`
- `apps/web/src/features/workspace/api/workspaceApi.ts`

## Prediction before reading/running

For `request<T>()`, predict what proves T at runtime. Compare a regular workspace JSON response with a versioned stream event parsed by an explicit runtime contract.

## Task

Trace BodySense generic request<T> from JSON into typed callers and identify where server contract/runtime validation provides (or fails to provide) evidence for T. Compare with schema parsing for stream events.

## Failure case

Change the server JSON field type while leaving the TypeScript generic unchanged. Explain why compilation can stay green and where runtime breakage appears.

## Verification command / evidence

- Trace `api-client.ts`, `workspaceApi.ts` and `stream-event-parser.ts`.
- Run stream parser tests and one workspace/service test; build a trust-boundary table for compile-time type vs runtime validator/server contract.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Does `response.json() as T` validate anything?
- Where should runtime validation live?
- When is generated/shared contract evidence sufficient, and when is schema parsing still valuable?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
