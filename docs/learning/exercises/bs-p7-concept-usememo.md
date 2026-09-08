# BS-P7-CONCEPT-USEMEMO · useMemo and cached derived values

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-USEMEMO**

## Concept

useMemo and cached derived values

## Prerequisites

- BS-P7-CONCEPT-HOOKS-MENTAL-MODEL

## BodySense target files

- `apps/web/src/features/consultation`
- `apps/web/src/features/body-explorer`

## Prediction before reading/running

Pick a `useMemo` call and predict dependency changes that recompute it versus renders that reuse the cached value.

## Task

Find a memoized or candidate expensive derivation, measure/observe rerenders or recomputation, and justify whether memoization is necessary.

## Failure case

Use useMemo as a semantic correctness mechanism or memoize trivial work while dependencies churn every render.

## Verification command / evidence

- Trace one `useMemo` in `DiagnosisPanel.tsx`, `BodyExplorer3D.tsx` or `BodyStateWorkbench.tsx`.
- Explain how you would measure whether memoization helps; do not change production code without evidence.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What guarantee does useMemo provide?
- Is memoization correctness or optimization?
- How can unstable dependencies defeat it?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
