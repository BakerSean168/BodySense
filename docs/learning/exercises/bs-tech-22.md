# BS-TECH-22 · authentication, session authority, role/capability and resource authorization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #22 · authentication middleware and authorization rules**

## Concept

authentication, session authority, role/capability and resource authorization

## Prerequisites

- TECH-21

## BodySense target files

- `apps/api/internal/middleware/auth.go`
- `apps/api/internal/middleware/auth_test.go`
- `apps/api/cmd/server/main.go`

## Prediction before reading/running

Predict middleware behavior for missing header, malformed/expired token, revoked session, Redis outage, valid member, and a role/capability-protected operation.

## Task

Trace `AuthMiddleware` and one additional authorization layer/capability. Produce a table separating identity validation, session liveness, role/capability checks and resource ownership.

## Failure case

Use tests or a controlled request to show that hiding a frontend button does not authorize an API. Exercise one identity failure and one permission/ownership failure.

## Verification command / evidence

- `cd apps/api && go test ./internal/middleware -count=1 -v`
- `Trace one protected route registration in `cmd/server/main.go`.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Authentication vs authorization?
- `Why does Redis/session outage fail closed for session-bound tokens?`
- Where should resource ownership be checked when it depends on a database row?

## Production change

none unless an API relies on frontend-only gating or lacks a server-side authority check.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
