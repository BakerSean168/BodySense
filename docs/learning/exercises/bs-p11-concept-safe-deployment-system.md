# BS-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM · a safe deployment system detects failure, preserves/recovers a known-good state and reports actionable evidence

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P11-CONCEPT-SAFE-DEPLOYMENT-SYSTEM**

## Concept

a safe deployment system detects failure, preserves/recovers a known-good state and reports actionable evidence.

## Prerequisites

- BS-P11-CONCEPT-CI-QUALITY-GATES

## BodySense target files

- `.github/workflows/deploy-production.yml`
- `.github/workflows/release-health.yml`
- `scripts/local-deploy-validate.sh`

## Prediction before reading/running

Trace preflight -> artifact/revision selection -> deploy -> health -> success/hold/rollback. Predict state when health fails halfway through activation.

## Task

Trace BodySense preflight -> deploy -> health -> rollback/hold behavior. State the invariant that prevents a failed candidate from silently becoming the only serving version.

## Failure case

Replace the serving version before the candidate passes health checks and provide no rollback/hold path.

## Verification command / evidence

- Trace `.github/workflows/deploy-production.yml`, `release-health.yml` and `scripts/local-deploy-validate.sh`.
- `pnpm test:delivery`; identify one explicit fail-closed/preflight invariant from code/tests.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes deploy retry safe?
- Liveness vs readiness vs smoke: which evidence is required?
- What state is preserved after failed activation?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
