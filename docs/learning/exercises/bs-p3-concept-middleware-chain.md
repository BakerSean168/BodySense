# BS-P3-CONCEPT-MIDDLEWARE-CHAIN · HTTP middleware chain and request/response cross-cutting concerns

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P3-CONCEPT-MIDDLEWARE-CHAIN**

## Concept

HTTP middleware chain and request/response cross-cutting concerns.

## Prerequisites

- BS-FSO-3.1

## BodySense target files

- `apps/api/cmd/server/main.go`
- `apps/api/internal/middleware`

## Prediction before reading/running

Predict the request order among recovery/logging/CORS/auth/route handlers and which middleware can terminate the request before the handler.

## Task

Trace a BodySense request through logging/recovery/auth or other middleware before its handler. Explain context mutation, short-circuiting and why middleware is ordered composition.

## Failure case

Move auth after a protected handler or let a middleware call next after already writing a denial. Predict the security/response failure.

## Verification command / evidence

- `cd apps/api && go test ./internal/middleware -count=1`
- Trace one protected request through middleware registration order and note context values/short-circuit points.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does middleware own versus the handler?
- Why does registration order change semantics?
- What evidence proves a denied request never reached the protected handler?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
