# BS-P7-CONCEPT-ERROR-BOUNDARY · React error boundaries isolate render-tree failures and provide fallback UI without catching every async/event error

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-ERROR-BOUNDARY**

## Concept

React error boundaries isolate render-tree failures and provide fallback UI without catching every async/event error

## Prerequisites

- BS-P5-CONCEPT-COMPONENT-TEST-RENDER

## BodySense target files

- `apps/web/src/components/errors/RouteErrorBoundary.tsx`
- `apps/web/src/components/errors/RouteErrorBoundary.test.tsx`

## Prediction before reading/running

Predict which subtree is replaced when a descendant throws during render and what UI remains available outside the boundary.

## Task

Trace BodySense RouteErrorBoundary behavior and test. Classify which render/loader errors it catches and which network/event-handler failures require separate handling.

## Failure case

Assume an error boundary catches async promise rejections or arbitrary event-handler failures without explicit handling.

## Verification command / evidence

- Read and run `apps/web/src/components/errors/RouteErrorBoundary.test.tsx`.
- Trace where `RouteErrorBoundary` is mounted relative to navigation/router UI.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Which errors do React error boundaries catch?
- Why place boundaries at chosen ownership/recovery points?
- How should async/data errors be handled instead?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
