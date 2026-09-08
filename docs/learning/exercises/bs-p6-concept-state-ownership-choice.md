# BS-P6-CONCEPT-STATE-OWNERSHIP-CHOICE · choose local state, URL, Context, Zustand or TanStack Query based on ownership/lifetime/synchronization

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P6-CONCEPT-STATE-OWNERSHIP-CHOICE**

## Concept

choose local state, URL, Context, Zustand or TanStack Query based on ownership/lifetime/synchronization.

## Prerequisites

- BS-FSO-0.5
- BS-FSO-0.6

## BodySense target files

- `apps/web/src`

## Prediction before reading/running

Classify at least ten values as local component state, URL state, server state/cache, shared client presentation state or durable domain truth before reading their implementations.

## Task

Classify ten representative BodySense values by owner and lifetime, then justify their current mechanism. Flag any duplicated durable/server state or over-globalized local state.

## Failure case

Copy server workspace data into a Zustand store and mutate it independently of TanStack Query/server revisions. Predict drift and stale-write bugs.

## Verification command / evidence

- Build an ownership/lifetime matrix using `apps/web/src` examples.
- For each of TanStack Query, Zustand, local state and route state, cite one BodySense value and why the owner is correct.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- When is global state actually justified?
- Why is cached server state not durable truth?
- What state should survive reload/navigation/reconnect?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
