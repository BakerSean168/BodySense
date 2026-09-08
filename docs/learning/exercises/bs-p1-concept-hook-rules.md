# BS-P1-CONCEPT-HOOK-RULES · Rules of Hooks and stable hook call order

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-HOOK-RULES**

## Concept

Rules of Hooks and stable hook call order

## Prerequisites

- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/consultation/hooks`
- `apps/web/src/features/workspace/hooks`

## Prediction before reading/running

Count the hook call order for a component/custom hook and predict why conditional calls would shift React slot identity.

## Task

Review one custom BodySense hook and explain why hooks are called unconditionally at component/hook top level; create a tiny invalid example only in a test/scratch context if needed.

## Failure case

Put a hook behind a condition that changes between renders and predict the state/effect slot mismatch.

## Verification command / evidence

- Trace hook call structure in `apps/web/src/features/consultation/hooks` or workspace hooks.
- Run `pnpm nx typecheck @bodysense/web` plus lint if needed; explain why static linting can catch rule violations before runtime.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why must hooks be called at top level?
- What does stable call order buy React?
- May a custom hook call hooks conditionally?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
