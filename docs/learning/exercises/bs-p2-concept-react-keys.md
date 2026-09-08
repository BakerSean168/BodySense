# BS-P2-CONCEPT-REACT-KEYS · React key identity for reconciling list items

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P2-CONCEPT-REACT-KEYS**

## Concept

React key identity for reconciling list items

## Prerequisites

- BS-P1-CONCEPT-JSX

## BodySense target files

- `apps/web/src/features/workspace/components`
- `apps/web/src/features/consultation/components`

## Prediction before reading/running

For a rendered list, identify the stable domain identity used as key and predict component identity after insertion/reordering.

## Task

Inspect one BodySense list key and explain why it represents stable item identity across reorders/updates rather than merely suppressing a warning.

## Failure case

Use array index as key, insert at the start and predict which child state/ref may now attach to the wrong item.

## Verification command / evidence

- Trace a mapped list in workspace/consultation components and identify its key source.
- Construct a small reasoning table for stable ID key versus index key under insert/delete/reorder.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, what it does not prove, and what observation would falsify the conclusion.

## Explain-back questions

- What does a key identify?
- Why is key not passed as a normal prop?
- When is an index key acceptable?

## Production change

No production change is required when the current design already satisfies the concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused code/test/runtime evidence, explain the failure case, and justify the relevant ownership/lifecycle/performance trade-off independently.
