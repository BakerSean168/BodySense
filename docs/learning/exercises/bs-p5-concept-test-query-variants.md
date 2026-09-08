# BS-P5-CONCEPT-TEST-QUERY-VARIANTS · Testing Library query timing and absence semantics

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-TEST-QUERY-VARIANTS**

## Concept

Testing Library getBy/findBy/queryBy variants encode synchronous presence, asynchronous appearance and expected absence semantics.

## Prerequisites

- BS-P5-CONCEPT-TESTING-LIBRARY-QUERIES

## BodySense target files

- `apps/web/src/components/__tests__/ProtectedRoute.test.tsx`
- `apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx`
- `apps/web/src/features/consultation/components/__tests__/StreamingAssistantTurn.test.tsx`

## Prediction before reading/running

For three assertions—present now, appears after async work, must remain absent—choose getBy/findBy/queryBy and predict how the wrong variant fails or times out.

## Task

Classify BodySense test queries by expected timing/presence: use getBy for synchronous presence, findBy for asynchronous appearance, and queryBy when absence is the assertion. Explain failure/timeout behavior for the wrong choice.

## Failure case

Use queryBy for required content and forget to assert non-null, or use getBy for asynchronously rendered content so the test fails before the UI has a chance to settle.

## Verification command / evidence

- Inspect the three target tests and classify each query by timing/presence contract.
- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/components/__tests__/ProtectedRoute.test.tsx apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx apps/web/src/features/consultation/components/__tests__/StreamingAssistantTurn.test.tsx`

Passing tests are not sufficient for L4. Explain why each chosen query distinguishes the intended behavior and what query choice could create a false positive/false negative.

## Explain-back questions

- What is the semantic difference between getBy, findBy and queryBy?
- Why does findBy return a Promise?
- Why is queryBy usually the right primitive for an absence assertion?

## Production change

No production change is required. Test changes are justified only when the current query semantics do not match the behavior being asserted.

## L4 acceptance

Complete only when the learner can choose the query family from behavior timing/presence before reading an existing answer, verify it in a focused test, and explain the failure mode of the alternatives.
