# BS-P9-CONCEPT-STRUCTURAL-TYPING · TypeScript compatibility is structural: values satisfy required shape regardless of nominal declaration identity

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-STRUCTURAL-TYPING**

## Concept

TypeScript compatibility is structural: values satisfy required shape regardless of nominal declaration identity.

## Prerequisites

- None

## BodySense target files

- `apps/web/src/features/workspace/types/workspace.ts`
- `packages/contracts/src/stream-events.ts`

## Prediction before reading/running

Given two independently declared object types with the same required shape, predict assignability. Then add an extra property, remove a required property and change one member type.

## Task

Compare two BodySense object types/values by required members. Predict assignments with extra/missing/incompatible properties and explain how structural typing differs from nominal class/interface identity.

## Failure case

Assume interface names create nominal identity and reject/accept values based on declaration name instead of shape.

## Verification command / evidence

- Use `workspace.ts` and `stream-events.ts` to compare real object shapes.
- Run `pnpm nx run web:typecheck` after a scratch/local type-only experiment, then revert the experiment.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does “structural” mean?
- Why can excess-property checking seem stricter for fresh object literals?
- How does structural typing help interoperate across modules?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
