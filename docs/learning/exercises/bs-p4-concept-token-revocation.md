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

Predict access/refresh behavior after normal refresh rotation, replay of a consumed refresh token and logout/revocation of a token family. Also predict the difference between a cryptographically valid access token and a server-side session that has already been revoked.

## Task

Compare stateless signed-token validity with BodySense server-side session authority. Trace access-token session checks, Redis-backed opaque refresh rotation/replay tombstones and family revocation, then compare Authorization-header transport with the source course cookie/session alternative and explain revocation-latency/per-request-lookup trade-offs.

## Failure case

Treat a cryptographically valid access/refresh token as sufficient authority until expiry even after server-side revocation, or rotate refresh tokens without replay-family invalidation. Explain how stolen credentials remain usable and how a cache outage must fail closed rather than silently restore authority.

## Verification command / evidence

- `cd apps/api && go test ./internal/service -run "RefreshToken|Logout|GenerateTokens" -count=1`
- `cd apps/api && go test ./internal/middleware -run "Session|Auth" -count=1`
- Build a comparison matrix: stateless JWT expiry vs server-side session authority; opaque refresh token family/replay; Authorization header vs cookie transport; immediate revocation vs lookup cost.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why rotate refresh tokens?
- What is a token family and why can replay revoke it?
- What authority is checked on every access request?
- What does server-side session state buy compared with a purely stateless JWT, and what per-request availability/performance cost does it introduce?
- Why are cookie vs Authorization header transport choices separate from the question of whether session authority is stateful?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
