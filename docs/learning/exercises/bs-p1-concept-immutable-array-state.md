# BS-P1-CONCEPT-IMMUTABLE-ARRAY-STATE · immutable updates of array-backed React state

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-IMMUTABLE-ARRAY-STATE**

## Concept

immutable updates of array-backed React state

## Prerequisites

- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `apps/web/src/features/body-explorer/model/bodyExplorerStore.ts`

## Prediction before reading/running

Predict the next array reference/content for an add/update/remove transition without mutating the previous snapshot.

## Task

Trace an append/update/filter of collection state and predict how in-place mutation could defeat change detection or corrupt prior snapshots.

## Failure case

Push/splice an array held by state and reuse the same reference; explain why change detection and historical reasoning become unreliable.

## Verification command / evidence

- Trace immutable collection transitions in `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`.
- Run `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/runtime/activeTurnReducer.test.ts`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why does reference identity matter?
- When are map/filter/spread appropriate?
- How does immutability help reducer/debugging semantics?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
