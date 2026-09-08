# BS-P2-CONCEPT-CONTROLLED-COMPONENT · controlled inputs: value supplied by state and changes returned through events

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P2-CONCEPT-CONTROLLED-COMPONENT**

## Concept

controlled inputs: value supplied by state and changes returned through events

## Prerequisites

- BS-P1-CONCEPT-EVENT-HANDLING
- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/auth/components/LoginForm.tsx`
- `apps/web/src/components`

## Prediction before reading/running

Trace one input value from React state to the DOM and its onChange event back into state.

## Task

Inspect a BodySense controlled input and explain the feedback loop `state -> value -> onChange -> setState -> render`, including validation implications.

## Failure case

Supply a fixed value without a working change path or mix controlled/uncontrolled ownership across the component lifetime.

## Verification command / evidence

- Trace `apps/web/src/features/auth/components/LoginForm.tsx`.
- Use browser/component evidence to show the typed value is controlled by React state and submit reads the same state.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Who owns a controlled input value?
- What makes an input uncontrolled?
- Why can switching ownership mid-lifecycle cause warnings/bugs?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
