# BS-P9-CONCEPT-UNKNOWN-NARROWING · unknown is the safe top type for uncertain values and must be narrowed before operations

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-UNKNOWN-NARROWING**

## Concept

unknown is the safe top type for uncertain values and must be narrowed before operations.

## Prerequisites

- BS-P9-CONCEPT-TYPE-ERASURE

## BodySense target files

- `packages/contracts/src/stream-event-parser.ts`
- `apps/web/src/features/consultation/runtime/threadMessageMapping.ts`

## Prediction before reading/running

Start with unknown and predict which operations are forbidden until typeof/in/Array.isArray/custom guards establish facts.

## Task

Start from unknown external input and narrow it using typeof/in/operator/shape checks. Explain why unknown preserves safety while any disables it.

## Failure case

Replace unknown with any to silence errors and then access a missing nested field. Explain why the compiler can no longer help.

## Verification command / evidence

- Trace narrowing in `stream-event-parser.ts` or `threadMessageMapping.ts`.
- Run parser/mapping unit tests and identify which malformed shapes are rejected.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- unknown vs any: what proof is required to use each?
- How does control-flow analysis remember guards?
- What makes a custom type guard trustworthy?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
