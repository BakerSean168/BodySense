# BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE · the same source revision should undergo the same deterministic checks/build inputs on every run

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P11-CONCEPT-REPRODUCIBLE-PIPELINE**

## Concept

the same source revision should undergo the same deterministic checks/build inputs on every run.

## Prerequisites

- BS-TECH-10

## BodySense target files

- `pnpm-lock.yaml`
- `.github/workflows/ci.yml`
- `apps/web/Dockerfile`
- `apps/api/Dockerfile`

## Prediction before reading/running

List every input that can make the same commit build differently: dependency versions, action refs, base images, environment/network/time and generated assets. Predict which BodySense controls are pinned.

## Task

Identify BodySense sources of nondeterminism (unpinned actions/dependencies/latest tags/time/network) and the mechanisms used to constrain them. Explain what “same thing happens every time” realistically means.

## Failure case

Use mutable `latest`/branch action refs and unpinned dependencies; later attempt to reproduce a failed release from the same commit.

## Verification command / evidence

- Inspect `.github/workflows/ci.yml`, lockfiles and Dockerfiles for pinned/reproducible inputs.
- `pnpm test:delivery`

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does reproducible mean when external services still exist?
- Which inputs should be recorded versus pinned?
- Why is the commit SHA alone not a build artifact identity?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
