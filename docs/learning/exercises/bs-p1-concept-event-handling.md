# BS-P1-CONCEPT-EVENT-HANDLING · React event handling from user action to state transition

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-EVENT-HANDLING**

## Concept

React event handling from user action to state transition

## Prerequisites

- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/auth/components/LoginForm.tsx`
- `apps/web/src/features/consultation/components`

## Prediction before reading/running

Trace one click/submit from DOM event through a React handler into state or command execution and predict its observable result.

## Task

Trace one click/change/submit event through its handler and resulting state/mutation, including event object use and side-effect boundary.

## Failure case

Call the handler during render instead of passing a function reference, causing immediate work or render loops.

## Verification command / evidence

- Trace submit behavior in `apps/web/src/features/auth/components/LoginForm.tsx`.
- Use a focused component test or browser interaction to verify the handler runs only after the user event.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does `onClick={fn}` mean versus `onClick={fn()}`?
- Where should preventDefault occur?
- How do event handlers differ from render and effects?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
