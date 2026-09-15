# BS-P7-CONCEPT-CUSTOM-HOOKS · custom hooks reuse stateful behavior and define feature-oriented lifecycle APIs

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-CUSTOM-HOOKS**

## Concept

custom hooks reuse stateful behavior and define feature-oriented lifecycle APIs

## Prerequisites

- BS-P7-CONCEPT-HOOKS-MENTAL-MODEL

## BodySense target files

- `apps/web/src/features/workspace/hooks`
- `apps/web/src/features/consultation/hooks`

## Prediction before reading/running

For a feature custom hook, predict which lifecycle/state concerns it encapsulates and what public API callers receive.

## Task

Trace one BodySense custom hook and separate reusable behavior from component rendering. Explain inputs, returned contract, dependencies and why hook names/call rules matter.

## Failure case

Extract code into a “hook” that still leaks every transport/cache detail or whose lifecycle ownership remains split across callers.

## Verification command / evidence

- Trace `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts` or consultation hooks from internal hooks to returned API.
- Run a focused hook test such as `useConversationActions.test.tsx` or `useBodyExplorerWorkspace.test.tsx`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What should a custom hook encapsulate?
- Can custom hooks share state automatically?
- When is a plain function/service better?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
