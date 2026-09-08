# BS-P9-CONCEPT-DISCRIMINATED-UNIONS · discriminated unions model variant-specific data while preserving shared fields and safe narrowing

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-DISCRIMINATED-UNIONS**

## Concept

discriminated unions model variant-specific data while preserving shared fields and safe narrowing.

## Prerequisites

- BS-P9-CONCEPT-STRUCTURAL-TYPING

## BodySense target files

- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `packages/contracts/src/stream-events.ts`

## Prediction before reading/running

For a tagged event/action union, predict which fields become available after checking its discriminant and which impossible field combinations cannot be constructed.

## Task

Trace a BodySense tagged union by its discriminant. Show how each branch exposes variant-specific fields and how an impossible combination is prevented at compile time.

## Failure case

Replace the tagged union with one interface full of optional fields. Explain invalid states that now type-check.

## Verification command / evidence

- Trace `packages/contracts/src/stream-events.ts` and `activeTurnReducer.ts`.
- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/runtime/activeTurnReducer.test.ts`

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why is a stable literal discriminant powerful?
- How do unions encode impossible states?
- When should two variants share a base type?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
