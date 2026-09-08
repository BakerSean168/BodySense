# BS-P1-CONCEPT-COMPONENT · React function components as reusable UI units

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-COMPONENT**

## Concept

React function components as reusable UI units

## Prerequisites

- BS-FSO-0.1

## BodySense target files

- `apps/web/src/components`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

Pick a BodySense function component and predict its rendered tree from props/state before reading child implementations.

## Task

Decompose one BodySense UI surface into component responsibilities and explain the boundary between component rendering and domain/server ownership.

## Failure case

Treat a component function like an imperative one-time render and mutate external state during render; explain duplicate/strict rendering consequences.

## Verification command / evidence

- Trace `apps/web/src/features/workspace/components/BodyStateWorkbench.tsx` from props to returned JSX.
- Run `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What makes a function a React component rather than an ordinary function?
- Why must render stay pure?
- Where should durable side effects live instead?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
