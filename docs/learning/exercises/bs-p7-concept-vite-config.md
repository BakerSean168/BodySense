# BS-P7-CONCEPT-VITE-CONFIG · Vite configuration defines plugins, aliases, proxy, asset base, dependency optimization and test environment

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-VITE-CONFIG**

## Concept

Vite configuration defines plugins, aliases, proxy, asset base, dependency optimization and test environment

## Prerequisites

- BS-P7-CONCEPT-VITE-DEV-PROD

## BodySense target files

- `apps/web/vite.config.ts`

## Prediction before reading/running

Before reading each non-default Vite block, predict its owner/risk: React plugin, aliases, allowed hosts, dev proxy, asset base, dependency dedupe, Vitest environment.

## Task

Read BodySense vite.config.ts and explain each non-default block, especially asset base, allowed hosts, /api proxy, aliases, dedupe and Vitest configuration.

## Failure case

Change one config knob without knowing whether it affects dev server, build output, tests or deployment and accidentally fix one environment while breaking another.

## Verification command / evidence

- Annotate each non-default block in `apps/web/vite.config.ts` with dev/build/test/runtime scope.
- Run web typecheck/build and one Vitest file after any hypothetical configuration change; no production config change is needed for the exercise.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Which Vite settings affect dev only?
- How are aliases shared with TypeScript resolution?
- Why can test configuration live alongside Vite config but still represent a different runtime?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
