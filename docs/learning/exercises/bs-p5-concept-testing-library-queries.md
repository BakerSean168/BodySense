# BS-P5-CONCEPT-TESTING-LIBRARY-QUERIES · Testing Library queries should prefer user-observable semantics over implementation details

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-TESTING-LIBRARY-QUERIES**

## Concept

Testing Library queries should prefer user-observable semantics over implementation details

## Prerequisites

- BS-P5-CONCEPT-COMPONENT-TEST-RENDER

## BodySense target files

- `apps/web/src/components/__tests__/ProtectedRoute.test.tsx`
- `apps/web/src/features/workspace/components/__tests__/BodyStateWorkbench.test.tsx`

## Prediction before reading/running

Choose the most user-semantic query for a visible control before looking at the existing test.

## Task

Compare getByRole/getByLabel/getByText/test-id style queries in BodySense tests and rank them by how closely they specify the user-facing contract.

## Failure case

Select elements by brittle DOM nesting/class selectors and make harmless markup refactors fail the test.

## Verification command / evidence

- Compare query choices in `ProtectedRoute.test.tsx` and `BodyStateWorkbench.test.tsx`.
- Run both focused tests and justify role/name/text/label query priority.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Why prefer role/name?
- When is getByTestId justified?
- What does a good query tell you about accessibility?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
