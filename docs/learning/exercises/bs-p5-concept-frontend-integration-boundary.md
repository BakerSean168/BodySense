# BS-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY · frontend integration tests combine components/state/transport mocks at a wider boundary than unit tests

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY**

## Concept

frontend integration tests combine components/state/transport mocks at a wider boundary than unit tests

## Prerequisites

- BS-P5-CONCEPT-COMPONENT-TEST-RENDER
- BS-P2-CONCEPT-PROMISES

## BodySense target files

- `apps/web/src/features/consultation/pages/__tests__/ConsultationPage.test.tsx`
- `apps/web/src/features/consultation/services/__tests__/consultationService.test.ts`

## Prediction before reading/running

For `ConsultationPage.test.tsx`, list which layers are real and which are mocked, and predict one bug the page-level test catches that a reducer/service unit test cannot.

## Task

Classify one BodySense frontend test as unit/component/integration and list real versus mocked dependencies. Explain what additional confidence the wider boundary buys.

## Failure case

Mock every child/hook/transport so the “integration” test only proves mocks call mocks, or run the entire stack when a narrower deterministic boundary would localize behavior better.

## Verification command / evidence

- Read `ConsultationPage.test.tsx` and `consultationService.test.ts` and classify real/mocked dependencies.
- Run both focused files and write a confidence matrix: component wiring/state/route/transport contract vs actual network/backend/database.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes a frontend test integration-level?
- What confidence is lost when a dependency is mocked?
- Why are wider tests slower/harder to localize but still valuable?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
