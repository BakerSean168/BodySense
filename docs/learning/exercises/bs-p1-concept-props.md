# BS-P1-CONCEPT-PROPS · props as immutable component inputs

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-PROPS**

## Concept

props as immutable component inputs

## Prerequisites

- BS-P1-CONCEPT-COMPONENT
- BS-P1-CONCEPT-JSX

## BodySense target files

- `apps/web/src/components`
- `apps/web/src/features/workspace/components`

## Prediction before reading/running

Choose a child component and predict which parent owns each prop value and whether the child may mutate it.

## Task

Choose one reusable BodySense component and trace each prop from parent expression to child use, including TypeScript inference/contract and why the child does not own the input.

## Failure case

Mutate an object received through props and explain aliasing, stale memoization and ownership bugs that can result.

## Verification command / evidence

- Trace a parent -> child prop path in `apps/web/src/features/workspace/components`.
- Run `pnpm nx typecheck @bodysense/web`.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why are props inputs rather than component-owned state?
- What is the difference between mutating a prop object and asking the owner to update state?
- How does TypeScript describe the public prop contract?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
