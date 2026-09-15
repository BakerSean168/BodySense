# BS-P5-CONCEPT-USER-EVENT-TESTING · interaction tests should drive UI through realistic user events and assert resulting behavior

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-USER-EVENT-TESTING**

## Concept

interaction tests should drive UI through realistic user events and assert resulting behavior

## Prerequisites

- BS-P1-CONCEPT-EVENT-HANDLING
- BS-P5-CONCEPT-TESTING-LIBRARY-QUERIES

## BodySense target files

- `apps/web/src`
- `apps/web/src/components`

## Prediction before reading/running

Predict the visible state/callback change after a realistic click/type/submit sequence.

## Task

Trace a BodySense button/input test from user interaction to callback/state/UI assertion. Explain why invoking implementation functions directly tests a different boundary.

## Failure case

Invoke component callbacks directly and claim to have tested the user interaction wiring.

## Verification command / evidence

- Pick a component interaction test under `apps/web/src/features` and identify the user-level event path.
- If an existing test uses lower-level fireEvent/direct callback, explain the trade-off and whether userEvent would better model the interaction.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why does user-event semantics matter?
- What layers are bypassed by direct callback invocation?
- Which interactions require awaiting?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
