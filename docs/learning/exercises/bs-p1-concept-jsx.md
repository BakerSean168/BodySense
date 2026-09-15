# BS-P1-CONCEPT-JSX · JSX as JavaScript expressions describing UI

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-JSX**

## Concept

JSX as JavaScript expressions describing UI

## Prerequisites

- BS-P1-CONCEPT-COMPONENT

## BodySense target files

- `apps/web/src/App.tsx`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

Translate one JSX fragment into the element tree/value relationships React receives; identify embedded JavaScript expressions.

## Task

Trace one JSX tree to the values/expressions it evaluates and the DOM it produces; distinguish JSX syntax from HTML and from runtime DOM nodes.

## Failure case

Insert an arbitrary non-renderable object into JSX and predict the runtime/type failure versus a valid element/string/number.

## Verification command / evidence

- Trace JSX in `apps/web/src/features/workspace/components/BodyStateWorkbench.tsx`.
- Run `pnpm nx typecheck @bodysense/web`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What is JSX at runtime?
- Which values are renderable children?
- Why are braces JavaScript expression boundaries, not template interpolation?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
