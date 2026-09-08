# BS-P1-CONCEPT-ASYNC-STATE-UPDATES · queued/asynchronous React state updates and stale snapshot reads

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-ASYNC-STATE-UPDATES**

## Concept

queued/asynchronous React state updates and stale snapshot reads

## Prerequisites

- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/consultation`
- `apps/web/src/features/workspace`

## Prediction before reading/running

For two queued updates, predict the result using value-form versus functional updater semantics.

## Task

Find or construct a BodySense local-state update where reading state immediately after setting it would still observe the current render snapshot; explain when functional updates are required.

## Failure case

Use a captured stale value for multiple dependent updates and lose one increment/transition.

## Verification command / evidence

- Locate a state update that depends on previous state in BodySense and explain whether functional update/reducer semantics are required.
- Use an isolated test or existing reducer test to distinguish stale-capture from correct transition behavior.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why can updates be batched?
- When is `setX(x + 1)` unsafe?
- How does reducer dispatch avoid some stale snapshot mistakes?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
