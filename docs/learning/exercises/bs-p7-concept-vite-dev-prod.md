# BS-P7-CONCEPT-VITE-DEV-PROD · Vite development native-ESM/HMR pipeline differs fundamentally from production Rollup bundle output

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-VITE-DEV-PROD**

## Concept

Vite development native-ESM/HMR pipeline differs fundamentally from production Rollup bundle output

## Prerequisites

- BS-P7-CONCEPT-BUNDLING

## BodySense target files

- `apps/web/vite.config.ts`
- `package.json`

## Prediction before reading/running

Predict the differences between `vite` dev serving/HMR and production `vite build` output for module loading, transformations, caching and API proxy behavior.

## Task

Compare BodySense npm/pnpm dev and build behavior: module serving/transforms/HMR versus optimized bundle/chunks and static asset base.

## Failure case

Validate only in dev and assume proxy/base/static-asset behavior will be identical in production.

## Verification command / evidence

- Compare root/web dev/build commands and `apps/web/vite.config.ts`.
- Run build plus a config trace; identify settings used only during dev versus production output/runtime.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why is Vite dev fast?
- What does HMR preserve/change?
- Which production-only failures can dev proxy hide?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
