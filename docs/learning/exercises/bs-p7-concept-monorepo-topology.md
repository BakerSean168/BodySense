# BS-P7-CONCEPT-MONOREPO-TOPOLOGY · frontend/backend can share one repository while retaining independent runtime/build boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-MONOREPO-TOPOLOGY**

## Concept

frontend/backend can share one repository while retaining independent runtime/build boundaries

## Prerequisites

- BS-P1-CONCEPT-COMPONENT
- BS-FSO-3.1

## BodySense target files

- `pnpm-workspace.yaml`
- `apps/web`
- `apps/api`
- `apps/ai-service`

## Prediction before reading/running

Predict which BodySense directories are independently built/deployed apps versus shared packages/config, and which contracts may cross those boundaries.

## Task

Map BodySense monorepo applications/packages, shared contracts and independent deployables. Explain advantages and coupling risks versus separate repositories.

## Failure case

Treat a monorepo as one runtime/process or let apps import private implementation code across service boundaries instead of explicit contracts.

## Verification command / evidence

- Trace `pnpm-workspace.yaml`, root `package.json`, `apps/web`, `apps/api`, `apps/ai-service` and shared packages.
- Run `pnpm nx show projects` / inspect the project graph and identify build/runtime boundaries versus repository co-location.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does a monorepo share, and what must remain separate?
- How can shared code create unwanted coupling?
- Why is repository topology different from deployment topology?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
