# BS-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE · browser credential persistence trade-offs: localStorage versus in-memory access token and cookie-backed refresh

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-BROWSER-TOKEN-PERSISTENCE**

## Concept

browser credential persistence trade-offs: localStorage versus in-memory access token and cookie-backed refresh.

## Prerequisites

- BS-P4-CONCEPT-TOKEN-REVOCATION

## BodySense target files

- `apps/web/src/stores/authStore.ts`
- `apps/web/src/features/auth/services/authService.ts`

## Prediction before reading/running

Before reading the store, predict what survives a page reload: in-memory access token, HttpOnly refresh cookie/session, user projection and localStorage.

## Task

Compare the course localStorage token approach with BodySense access-token-in-memory plus credentialed refresh flow. Explain reload persistence, XSS exposure, HttpOnly-cookie boundaries and logout/revocation consequences.

## Failure case

Persist bearer access/refresh material in JavaScript-readable storage and then introduce XSS. Explain the resulting credential theft path.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/stores/authStore.test.ts`
- Trace bootstrap/login/refresh/logout across `authStore.ts` and `authService.ts`; record which credential is readable by JavaScript.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why can an HttpOnly cookie still have CSRF concerns?
- Why keep short-lived access state in memory?
- What should happen when bootstrap refresh fails?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
