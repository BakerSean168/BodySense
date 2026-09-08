# BS-P1-CONCEPT-RENDER-CYCLE · React render snapshots and state-triggered rerendering

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-RENDER-CYCLE**

## Concept

React render snapshots and state-triggered rerendering

## Prerequisites

- BS-P1-CONCEPT-COMPONENT

## BodySense target files

- `apps/web/src/features/consultation/context/ActiveTurnContext.tsx`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

For one local/state-driven component, predict which code runs again after a state update and what values belong to the old versus new render snapshot.

## Task

Choose one state update and predict which component function runs again, which values belong to the old snapshot, and why rerender is not equivalent to remount.

## Failure case

Read state immediately after scheduling an update and assume the current closure has changed; explain the stale snapshot result.

## Verification command / evidence

- Trace a state-driven render in `apps/web/src/features/workspace/components/BodyStateWorkbench.tsx` or consultation UI.
- Run the BodyStateWorkbench focused test and identify the observable rerender transition.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What triggers a render?
- Why is each render a snapshot?
- What remains stable across renders and what is recreated?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
