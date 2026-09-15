# BS-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE · production must expose which source revision/artifact is actually running

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P11-CONCEPT-DEPLOYED-REVISION-PROVENANCE**

## Concept

production must expose which source revision/artifact is actually running.

## Prerequisites

- BS-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM

## BodySense target files

- `.github/workflows/deploy-production.yml`
- `.github/workflows/release-health.yml`
- `docs/architecture/deployment-architecture.md`

## Prediction before reading/running

Starting from a running production instance, predict how to recover semantic release version, commit SHA and immutable artifact/image identity.

## Task

Trace BodySense commit SHA/version/image identity into deployment/health evidence. Explain why “main is green” is insufficient if the deployed revision cannot be identified.

## Failure case

Only record “main” or “latest” in deployment logs. Explain why incident rollback/audit cannot prove what ran.

## Verification command / evidence

- Trace revision/version/image identity through release/deploy/health workflow or manifest files.
- Use delivery manifest tests (`pnpm test:delivery`) to identify fields that bind source to artifact/deployment.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Release version vs commit SHA vs image digest: what does each identify?
- Why must health evidence name the revision it checked?
- How does provenance support rollback?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
