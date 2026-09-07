# BS-TECH-10 · CI database lanes as release evidence

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #10 · GitHub Actions with Go/Postgres tests**

## Concept

CI database lanes as release evidence

## Prerequisites

- TECH-05

## BodySense target files

- `.github/workflows/ci.yml`
- `scripts/delivery/generate-manifest.mjs`
- `scripts/delivery/run-quality.mjs`
- `apps/api/cmd/migration-validator`

## Prediction before reading/running

Predict which repository changes enable database-current, database-production or generic quality lanes, and which job/oracle ultimately gates the workflow.

## Task

Trace the BodySense CI path for a database-related change from delivery manifest -> selected child jobs -> migration/domain validators -> oracle result. Draw dependencies and explain why the release gate is conditional rather than always running everything.

## Failure case

Model a migration that fails only on the production baseline and a quality test that fails only in Go. Predict which child/oracle fails and whether other skipped lanes can mask the failure.

## Verification command / evidence

- `Read `.github/workflows/ci.yml` and the manifest selection scripts`
- Run the relevant local validation command for a small non-production case; do not trigger cloud deployment merely for the lesson.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why are selection logic and oracle logic separate?
- What evidence does a green unit-test lane not provide about migrations?
- How can conditional CI accidentally create a false green?

## Production change

none unless a lane-selection/validation gap is demonstrated by a test of the delivery scripts.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
