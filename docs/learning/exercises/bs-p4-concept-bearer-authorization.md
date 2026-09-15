# BS-P4-CONCEPT-BEARER-AUTHORIZATION · bearer token authentication resolves a principal before protected resource mutation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P4-CONCEPT-BEARER-AUTHORIZATION**

## Concept

bearer token authentication resolves a principal before protected resource mutation.

## Prerequisites

- BS-TECH-22
- BS-P3-CONCEPT-MIDDLEWARE-CHAIN

## BodySense target files

- `apps/api/internal/middleware/auth.go`
- `apps/api/internal/auth`
- `apps/api/internal/handler`

## Prediction before reading/running

Trace a bearer request: token parse -> signature/claims -> session authority -> principal context -> resource ownership check. Predict where 401 differs from 403.

## Task

Trace an authenticated BodySense request from Authorization header through token/session validation into user context and a protected handler. Distinguish authentication from resource authorization.

## Failure case

Use a valid token from user A against user B resource, or a token with a revoked/missing session. Predict the first rejection boundary.

## Verification command / evidence

- `cd apps/api && go test ./internal/middleware -run "Auth|Require" -count=1`
- Trace one user-owned handler/service and show where resource authorization occurs after authentication.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Authentication vs authorization: what fact does each establish?
- Why is a valid JWT not sufficient proof of current session authority?
- Where must owner ID come from?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
