# BS-P4-CONCEPT-TOKEN-REVOCATION · token/session revocation, expiry and replay handling are part of authentication authority

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P4-CONCEPT-TOKEN-REVOCATION**

## Concept

token/session revocation, expiry and replay handling are part of authentication authority.

## Prerequisites

- BS-TECH-37
- BS-P4-CONCEPT-BEARER-AUTHORIZATION

## BodySense target files

- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/middleware/auth.go`
- `apps/api/internal/service/auth_service_test.go`

## Prediction before reading/running

Predict access/refresh behavior after normal refresh rotation, replay of a consumed refresh token and logout/revocation of a token family.

## Task

Trace BodySense refresh/access token lifetime and revocation/replay behavior. Explain why signed token validity alone is insufficient when access must be revoked before token expiry.

## Failure case

Treat a signed refresh token as valid until expiry even after replay or logout. Explain how a stolen token remains usable.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "RefreshToken|Logout|GenerateTokens" -count=1`
- `cd apps/api && go test ./internal/middleware -run "Session|Auth" -count=1`

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why rotate refresh tokens?
- What is a token family and why can replay revoke it?
- What authority is checked on every access request?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
