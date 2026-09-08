# BS-P1-CONCEPT-STATE-PROP-OWNERSHIP · lifting/owning state and passing state down through props

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P1-CONCEPT-STATE-PROP-OWNERSHIP**

## Concept

lifting/owning state and passing state down through props

## Prerequisites

- BS-P1-CONCEPT-PROPS
- BS-P1-CONCEPT-USESTATE

## BodySense target files

- `apps/web/src/features/consultation/context`
- `apps/web/src/features/workspace`

## Prediction before reading/running

For shared UI data, identify the nearest legitimate owner and predict which components receive data versus callbacks.

## Task

Pick state consumed by multiple BodySense children, identify its actual owner, and explain why children receive values/actions rather than duplicate the state.

## Failure case

Store the same authoritative value independently in parent and child and let the copies drift.

## Verification command / evidence

- Build an ownership trace through `apps/web/src/features/consultation/context` or workspace components.
- Explain one case where lifting state is better and one where server/query state should not be lifted.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- Who owns a value?
- When should state be lifted?
- Why is globalizing state not the default answer?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
