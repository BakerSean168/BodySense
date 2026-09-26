# BS-TECH-21 · login credential verification and session/token issuance

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #21 · login API returns access token**

## Concept

login credential verification and session/token issuance

## Prerequisites

- TECH-20

## BodySense target files

- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/transport/httpapi/auth.go`
- `apps/api/internal/repository/user_repository.go`

## Prediction before reading/running

Predict the exact sequence for valid login: user lookup -> password verification -> last-login side effect -> access/refresh generation -> Redis/session authority -> response. State which failure messages are intentionally ambiguous.

## Task

Trace BodySense login end to end from HTTP handler into `AuthService.Login` and token/session creation. Mark which operations are security-critical and which best-effort side effects may fail without rejecting login.

## Failure case

Model nonexistent user, wrong password, Redis/session-authority unavailable, and last-login timestamp failure. Predict which cases fail closed and which continue.

## Verification command / evidence

- `cd apps/api && go test ./internal/handler ./internal/service -run "Login|Auth" -count=1 -v`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is password failure intentionally indistinguishable from unknown email?
- Why must session authority succeed before credentials are returned?
- Why can last-login metadata be best effort?

## Production change

none unless the fail-closed session/token invariant is violated.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
