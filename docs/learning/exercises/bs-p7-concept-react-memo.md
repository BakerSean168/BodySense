# BS-P7-CONCEPT-REACT-MEMO · React.memo and component rerender boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-REACT-MEMO**

## Concept

React.memo and component rerender boundaries

## Prerequisites

- BS-P1-CONCEPT-RENDER-CYCLE
- BS-P7-CONCEPT-USEMEMO
- BS-P7-CONCEPT-USECALLBACK

## BodySense target files

- `apps/web/src/features/consultation/components`
- `apps/web/src/features/body-explorer/components`

## Prediction before reading/running

Pick a frequently rendered component and predict which prop identities would have to remain stable for React.memo to skip a rerender.

## Task

Use React DevTools/test instrumentation to distinguish correctness from rerender optimization and justify a memo boundary only with evidence.

## Failure case

Wrap components in React.memo without profiling while passing fresh objects/callbacks each render, adding comparison/dependency complexity with no performance gain.

## Verification command / evidence

- Inspect consultation/body-explorer component boundaries and identify a plausible memo candidate plus unstable-prop risks.
- Use React DevTools/profiler or test instrumentation if a real performance issue exists; otherwise conclude that no memo boundary is currently justified.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does React.memo compare?
- Why can new object/function props defeat it?
- Why is memoization an optimization rather than a correctness tool?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
