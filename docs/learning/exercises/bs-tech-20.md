# BS-TECH-20 · JWT claims, signature algorithm, TTL and session binding

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #20 · create/verify JWT/PASETO token**

## Concept

JWT claims, signature algorithm, TTL and session binding

## Prerequisites

- TECH-15

## BodySense target files

- `apps/api/internal/auth/jwt.go`
- `apps/api/internal/auth/jwt_test.go`

## Prediction before reading/running

Predict the claims in a BodySense access token, the signing algorithm, expiry behavior, and what happens with a wrong secret or malformed token.

## Task

Trace `GenerateAccessToken` and `ValidateAccessToken`. Explain the difference between cryptographic token validity and live session authority, and why an explicit signing-method check exists.

## Failure case

Use existing tests for invalid token/wrong secret and reason about an algorithm-confusion attempt. Then explain why a cryptographically valid token can still be rejected by middleware after session revocation.

## Verification command / evidence

- `cd apps/api && go test ./internal/auth -run "GenerateAccessToken|ValidateAccessToken" -count=1 -v`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What does the JWT signature prove and not prove?
- Why bind `SessionID` into claims?
- Why is refresh credential design separate from access-token JWT validation?

## Production change

none unless a cryptographic/claims validation gap is demonstrated.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
