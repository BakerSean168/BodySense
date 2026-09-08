# BS-P2-CONCEPT-PROMISES · Promise pending/fulfilled/rejected states and chaining

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P2-CONCEPT-PROMISES**

## Concept

Promise pending/fulfilled/rejected states and chaining

## Prerequisites

- BS-P2-CONCEPT-ASYNC-RUNTIME

## BodySense target files

- `apps/web/src/lib/api-client.ts`
- `apps/web/src/features/workspace/api/workspaceApi.ts`

## Prediction before reading/running

For one request promise, predict pending -> fulfilled/rejected transitions and where thrown/rejected errors propagate.

## Task

Trace a successful and rejected BodySense request promise, then explain how errors propagate through the service/query layer.

## Failure case

Forget to await/return a promise chain and allow rejection or sequencing to escape the intended error boundary.

## Verification command / evidence

- Trace request/error flow in `apps/web/src/lib/api-client.ts`.
- Run a focused web service/hook test that covers a rejected request; identify the exact catch/propagation boundary.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What are the three promise states?
- How do await and then/catch relate?
- What happens when a callback throws?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
