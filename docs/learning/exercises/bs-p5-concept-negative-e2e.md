# BS-P5-CONCEPT-NEGATIVE-E2E · negative E2E paths assert rejection plus safe unchanged state and usable user feedback

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-NEGATIVE-E2E**

## Concept

negative E2E paths assert rejection plus safe unchanged state and usable user feedback

## Prerequisites

- BS-P5-CONCEPT-E2E-BLACK-BOX
- BS-P4-CONCEPT-BEARER-AUTHORIZATION

## BodySense target files

- `apps/web/src/features/auth`
- `apps/api/internal/handler/auth_handler.go`

## Prediction before reading/running

For invalid login or unauthorized protected navigation, predict HTTP/UI state, visible feedback, URL behavior and the absence of authenticated durable/client state.

## Task

Specify failed-login E2E expectations: status-facing behavior, visible error, protected navigation denial and absence of authenticated state.

## Failure case

Assert only that an error message appears while the app still stores credentials, enters the protected route, or mutates server state.

## Verification command / evidence

- Trace auth error handling in `apps/web/src/features/auth` and API auth handler tests.
- Design a Playwright negative-path assertion set (rejection + unchanged/safe state + usable feedback); add it only if no existing E2E covers the risk and the environment can be deterministic.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why must negative tests assert unchanged state?
- What is the difference between authentication failure and authorization failure?
- Which evidence proves the app remains usable after rejection?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
