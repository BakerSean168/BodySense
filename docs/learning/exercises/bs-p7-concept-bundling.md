# BS-P7-CONCEPT-BUNDLING · bundling resolves a module dependency graph into deployable browser assets/chunks

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-BUNDLING**

## Concept

bundling resolves a module dependency graph into deployable browser assets/chunks

## Prerequisites

- BS-P7-CONCEPT-TRANSPILATION

## BodySense target files

- `apps/web/vite.config.ts`
- `apps/web/Dockerfile`

## Prediction before reading/running

From `main.tsx`, predict that production build walks module imports and emits optimized hashed assets/chunks rather than mirroring one source file per module.

## Task

Trace BodySense web entry/module imports through Vite production build to hashed assets. Explain dependency graph, code splitting and why source modules are not shipped unchanged as one-to-one files.

## Failure case

Assume development native-ESM module serving is the same artifact topology as production, or rely on source-relative paths that break after hashing/base-path deployment.

## Verification command / evidence

- Run `pnpm nx build @bodysense/web` and inspect `apps/web/dist` asset/chunk names and entry HTML.
- Trace one lazy/imported module or dependency from source graph to emitted production assets.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What is the bundler input graph?
- Why are content hashes useful?
- What determines chunk boundaries?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
