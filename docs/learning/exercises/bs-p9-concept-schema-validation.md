# BS-P9-CONCEPT-SCHEMA-VALIDATION · schema validators provide runtime parsing/error reporting and can align inferred static types with trusted parsed output

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P9-CONCEPT-SCHEMA-VALIDATION**

## Concept

schema validators provide runtime parsing/error reporting and can align inferred static types with trusted parsed output.

## Prerequisites

- BS-P9-CONCEPT-TYPED-SERVER-DATA
- BS-P9-CONCEPT-UNKNOWN-NARROWING

## BodySense target files

- `packages/contracts/schemas/stream-event.v1.schema.json`
- `packages/contracts/src/stream-event-parser.ts`
- `apps/api/internal/dto`

## Prediction before reading/running

Given malformed stream/event input, predict schema/manual parser errors and the type available only after successful parse.

## Task

Compare Zod-style schema parsing with BodySense JSON Schema/manual parser/Go validation. Identify parse result, validation errors and how parsed output earns a trustworthy static type.

## Failure case

Allow unknown/missing discriminant or wrong field type to pass and let reducers consume it as a trusted event.

## Verification command / evidence

- `pnpm exec vitest run packages/contracts/src/stream-event-parser.test.ts` (or `pnpm --filter` equivalent if direct Vitest resolution requires it)
- Compare JSON Schema, manual parser and Go DTO validation for the same trust-boundary concerns.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Schema validation vs business validation: what differs?
- Why infer/static types from validated output?
- How should versioned schemas evolve?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
