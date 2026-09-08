# BS-P3-CONCEPT-SAME-ORIGIN-CORS · browser same-origin policy and CORS response authorization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P3-CONCEPT-SAME-ORIGIN-CORS**

## Concept

browser same-origin policy and CORS response authorization.

## Prerequisites

- BS-FSO-0.5
- BS-P3-CONCEPT-MIDDLEWARE-CHAIN

## BodySense target files

- `apps/api/cmd/server/main.go`
- `apps/web/vite.config.ts`

## Prediction before reading/running

For a browser at one scheme/host/port calling the API at another, predict whether the browser sends a preflight and which response headers must permit the real request.

## Task

Explain origin as scheme+host+port, then trace BodySense development/production cross-origin behavior and the server CORS policy. Distinguish browser enforcement from API authentication/authorization.

## Failure case

Assume CORS allows an origin but the API has no authentication, or authentication is correct but CORS blocks the browser. Explain why these are different failure/security layers.

## Verification command / evidence

- Inspect BodySense CORS middleware/registration and `apps/web/vite.config.ts` proxy configuration.
- Write a request matrix for same-origin dev proxy, direct cross-origin browser request, and non-browser API client.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What exactly constitutes an origin?
- Who enforces CORS?
- Why is CORS not authorization?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
