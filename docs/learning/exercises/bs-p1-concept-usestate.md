# BS-P1-CONCEPT-USESTATE · useState as component-local memory across renders

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-USESTATE**

## Concept

useState as component-local memory across renders

## Prerequisites

- BS-P1-CONCEPT-RENDER-CYCLE

## BodySense target files

- `apps/web/src/features/auth/components/LoginForm.tsx`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

For a `useState` call, predict initial value, update trigger, next render value and what the current closure still sees.

## Task

Trace one BodySense local state tuple from initialization through setter to rerender and justify why it is local state rather than URL/server/Zustand state.

## Failure case

Derive durable/server state into duplicated local state and let the copies diverge.

## Verification command / evidence

- Trace local form/UI state in `apps/web/src/features/auth/components/LoginForm.tsx`.
- Run `pnpm nx typecheck @bodysense/web`; if behavior is unclear, write a minimal isolated state-transition test before changing production code.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does useState persist between renders?
- Why should derived values often not be state?
- When is local state the wrong owner?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
