# BS-P7-CONCEPT-TRANSPILATION · transpilation converts source syntax/types/JSX to target JavaScript without changing intended semantics

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-TRANSPILATION**

## Concept

transpilation converts source syntax/types/JSX to target JavaScript without changing intended semantics

## Prerequisites

- BS-P1-CONCEPT-JSX
- BS-P9-CONCEPT-STRUCTURAL-TYPING

## BodySense target files

- `apps/web/vite.config.ts`
- `apps/web/tsconfig.json`

## Prediction before reading/running

Take a TSX source construct and predict what disappears/transforms before the browser runs it: TypeScript types, JSX syntax, modern target syntax.

## Task

Trace TypeScript/JSX source in BodySense to browser-targeted JavaScript and distinguish transpilation from type checking, bundling and minification.

## Failure case

Assume successful transpilation proves type correctness, or assume TypeScript runtime validation survives into emitted JavaScript.

## Verification command / evidence

- Trace `apps/web/tsconfig.json` and Vite React transform configuration.
- Run `pnpm nx typecheck @bodysense/web` separately from `pnpm nx build @bodysense/web` and explain why they prove different properties.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What is transpilation?
- What does type erasure mean here?
- How is transpilation different from bundling/minification/type checking?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
