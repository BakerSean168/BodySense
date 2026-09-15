# BS-P7-CONCEPT-HOOKS-MENTAL-MODEL · React hooks attach state/effect/ref/memoized behavior to function component render order

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-HOOKS-MENTAL-MODEL**

## Concept

React hooks attach state/effect/ref/memoized behavior to function component render order

## Prerequisites

- BS-P1-CONCEPT-HOOK-RULES
- BS-P2-CONCEPT-EFFECTS

## BodySense target files

- `apps/web/src`

## Prediction before reading/running

For one custom hook, classify each hook call as state/effect/ref/memo/callback and state what lifecycle or identity it contributes.

## Task

Classify hooks used in one BodySense component by state/effect/context/ref/memoization purpose and explain the stable call-order rule across renders.

## Failure case

Think of hooks as ordinary mutable objects independent of render order and expect them to survive arbitrary conditional calls.

## Verification command / evidence

- Trace a feature hook in `apps/web/src/features/workspace/hooks` or consultation hooks.
- Draw render -> hook slots -> effect/cleanup -> next-render sequence for that hook.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- How are hooks tied to render order?
- What is a stale closure?
- What makes a custom hook different from a service function?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
