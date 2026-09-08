# BS-P3-CONCEPT-HTTP-ERROR-TAXONOMY · HTTP error taxonomy: malformed input, invalid identifiers, absent resources and internal failures

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P3-CONCEPT-HTTP-ERROR-TAXONOMY**

## Concept

HTTP error taxonomy: malformed input, invalid identifiers, absent resources and internal failures.

## Prerequisites

- BS-FSO-3.1
- BS-P3-CONCEPT-MIDDLEWARE-CHAIN

## BodySense target files

- `apps/api/internal/handler/utils.go`
- `apps/api/internal/handler`

## Prediction before reading/running

Given malformed JSON, invalid ID syntax, missing resource, stale revision, unauthenticated request, forbidden ownership and DB failure, predict the status family and layer that detects each.

## Task

Trace representative BodySense 400/401/403/404/409/500 paths. Explain which layer creates the domain error and which layer maps it to an HTTP contract without leaking internal details.

## Failure case

Map every failure to 500 or leak an internal DB error string. Explain the client and security costs.

## Verification command / evidence

- Trace `apps/api/internal/handler/utils.go` plus representative handlers.
- Run focused handler/service tests for at least one 400/404/409/500 path and record domain error -> HTTP mapping.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Which errors are client-correctable?
- Why should domain errors not directly depend on HTTP status?
- What information belongs in logs but not the public error body?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
