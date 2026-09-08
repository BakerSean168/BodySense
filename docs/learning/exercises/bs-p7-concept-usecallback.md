# BS-P7-CONCEPT-USECALLBACK · useCallback and function identity dependencies

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-USECALLBACK**

## Concept

useCallback and function identity dependencies

## Prerequisites

- BS-P7-CONCEPT-HOOKS-MENTAL-MODEL

## BodySense target files

- `apps/web/src/features/consultation/hooks`
- `apps/web/src/features/workspace/hooks`

## Prediction before reading/running

Pick a `useCallback` and predict when its function identity changes and why any consumer cares.

## Task

Trace one callback passed across a component/hook boundary and verify whether stable identity affects an effect, memo, or child rerender.

## Failure case

Wrap every function in useCallback without an identity-sensitive consumer, adding dependency complexity for no benefit.

## Verification command / evidence

- Trace one `useCallback` from `ConsultationPage.tsx` or `useWorkspaceInvalidation.ts` into its consumer/dependencies.
- Explain the concrete identity-sensitive boundary or conclude the callback may be unnecessary; no production change without measurement/regression evidence.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does useCallback cache?
- When does function identity matter?
- How do stale dependencies create bugs?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
