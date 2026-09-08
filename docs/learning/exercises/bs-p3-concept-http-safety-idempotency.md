# BS-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY · HTTP method safety and idempotency as API contract properties

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P3-CONCEPT-HTTP-SAFETY-IDEMPOTENCY**

## Concept

HTTP method safety and idempotency as API contract properties.

## Prerequisites

- BS-FSO-3.1

## BodySense target files

- `apps/api/cmd/server/main.go`
- `apps/api/internal/handler`

## Prediction before reading/running

Classify GET, POST, PATCH and DELETE before reading handlers: which are safe, which should be idempotent, and which retry can duplicate a durable effect?

## Task

Classify representative BodySense GET/POST/PATCH/DELETE operations by safety and idempotency. Explain retry consequences and why method choice is a behavioral contract, not naming style.

## Failure case

Retry the same mutation after a timeout where the client cannot tell whether the first request committed. Explain duplicate-effect risk and what idempotency identity/expected revision would change.

## Verification command / evidence

- Trace one GET and two mutation routes in `apps/api/cmd/server/main.go` through handlers/services.
- Use an existing stale-revision/idempotency test where available; record the HTTP method, durable effect and retry consequence.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why are safety and idempotency different properties?
- Why does POST not automatically mean non-idempotent, and PUT/PATCH not automatically mean safe to retry?
- Which BodySense writes use revision/source identity to make retry behavior explicit?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
