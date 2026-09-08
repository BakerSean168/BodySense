# BS-P9-CONCEPT-EXHAUSTIVE-NARROWING · control-flow narrowing plus exhaustive switch checks make unhandled union variants visible at compile time

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-EXHAUSTIVE-NARROWING**

## Concept

control-flow narrowing plus exhaustive switch checks make unhandled union variants visible at compile time.

## Prerequisites

- BS-P9-CONCEPT-DISCRIMINATED-UNIONS

## BodySense target files

- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `packages/contracts/src/stream-events.ts`

## Prediction before reading/running

Add a hypothetical new union variant mentally: identify every switch/mapper that should fail compile-time if exhaustive handling is encoded correctly.

## Task

Walk a BodySense switch over a union. Add a hypothetical new variant and predict which exhaustive checks fail; explain never/assertNever-style verification.

## Failure case

Use a default branch that silently ignores unknown/new variants in a safety-critical reducer. Predict forward-compatibility versus silent-loss trade-offs.

## Verification command / evidence

- Trace a BodySense union switch/reducer and note the terminal narrowed type.
- Run web typecheck and focused reducer tests; document where a new variant would be caught and where runtime versioning must still handle it.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What role does `never` play?
- When is a permissive default branch intentional?
- Compile-time exhaustiveness vs runtime unknown events: why are both needed?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
