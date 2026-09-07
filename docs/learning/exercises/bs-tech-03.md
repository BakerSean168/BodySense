# BS-TECH-03 · versioned schema migration, baseline and replay

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #3 · database migrations**

## Concept

versioned schema migration, baseline and replay

## Prerequisites

- TECH-01

## BodySense target files

- `apps/api/migrations`
- `apps/api/cmd/migration-validator`
- `scripts/setup-postgres18-client-wrappers.sh`
- `.github/workflows/ci.yml`

## Prediction before reading/running

Predict how an empty database reaches current schema, how a production v29 baseline reaches current schema, and what failure should happen if a migration cannot apply cleanly.

## Task

Trace migration numbering, up/down files, production baseline handling and CI migration validation. Use a disposable PostgreSQL database to explain current-history and production-baseline validation paths.

## Failure case

Model an incompatible migration or a migration that succeeds from empty history but fails from production baseline. Identify which CI lane is designed to catch each case.

## Verification command / evidence

- `Use the existing migration-validator command pattern from `.github/workflows/ci.yml` against disposable PostgreSQL`
- `Do not run destructive down/replay against production.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is an empty-DB migration test insufficient for production upgrades?
- What is a baseline and why is it pinned?
- When is a down migration useful versus unsafe?

## Production change

no migration change is required; missing validation coverage can justify a test/tool improvement.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
