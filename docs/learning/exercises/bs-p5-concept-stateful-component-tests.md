# BS-P5-CONCEPT-STATEFUL-COMPONENT-TESTS · stateful component tests assert visible transitions and callback effects, not private state

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-STATEFUL-COMPONENT-TESTS**

## Concept

stateful component tests assert visible transitions and callback effects, not private state

## Prerequisites

- BS-P1-CONCEPT-USESTATE
- BS-P5-CONCEPT-USER-EVENT-TESTING

## BodySense target files

- `apps/web/src/features/consultation/components/__tests__`
- `apps/web/src/features/workspace/components/__tests__`

## Prediction before reading/running

Predict the before/after DOM for a stateful interaction without referring to internal state variables.

## Task

Choose a togglable/modal/panel-style BodySense component and specify initial visibility, interaction, next visibility and callback expectations without reaching into private state.

## Failure case

Read or mutate private state directly in the test and miss broken event/render integration.

## Verification command / evidence

- Use `BodyStateWorkbench.test.tsx` or a consultation component test to trace user event -> state transition -> visible output.
- Run the selected test and identify a falsifying assertion.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why should tests observe behavior rather than hook state?
- What makes a stateful component test robust?
- When should state logic instead be extracted and unit-tested separately?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
