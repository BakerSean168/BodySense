# BS-P7-CONCEPT-BROKEN-AUTHZ · broken authentication/access control and server-side authorization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-BROKEN-AUTHZ**

## Concept

broken authentication/access control and server-side authorization.

## Prerequisites

- BS-P4-CONCEPT-BEARER-AUTHORIZATION

## BodySense target files

- `apps/api/internal/middleware`
- `apps/api/internal/service/auth_service.go`
- `apps/api/internal/handler`

## Prediction before reading/running

Pick a UI action hidden for non-owners/operators and predict whether a direct API call with the same identity is rejected server-side.

## Task

Prove that hiding a frontend control is not authorization by exercising a protected API with missing identity and with the wrong resource owner.

## Failure case

Remove only the frontend button for unauthorized users while leaving the API route writable. Explain why the system remains vulnerable.

## Verification command / evidence

- `cd apps/api && go test ./internal/middleware -run "RequireKnowledgeOperator|Auth" -count=1`
- Trace one user-owned or operator-only service/handler and identify the server-side authorization condition.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why is UI visibility not an access-control boundary?
- What is IDOR/BOLA in a user-owned resource API?
- Which layer should enforce role versus resource ownership?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
