# BS-P5-CONCEPT-COMPONENT-TEST-RENDER · component tests render UI in a controlled DOM environment and assert observable output

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-COMPONENT-TEST-RENDER**

## Concept

component tests render UI in a controlled DOM environment and assert observable output

## Prerequisites

- BS-P1-CONCEPT-COMPONENT

## BodySense target files

- `apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx`
- `apps/web/src/test-setup.ts`

## Prediction before reading/running

Before reading assertions, predict the observable DOM output for one component test setup.

## Task

Read one BodySense component test from render to assertion. Identify test DOM setup, required providers/mocks and the user-visible contract being asserted.

## Failure case

Assert private implementation state/functions instead of user-visible output, making refactors break tests without behavior change.

## Verification command / evidence

- Read and run `apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx`.
- Explain what environment `apps/web/src/test-setup.ts` provides.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does a component test actually render?
- What should be asserted?
- Where is the boundary between unit and integration test?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
