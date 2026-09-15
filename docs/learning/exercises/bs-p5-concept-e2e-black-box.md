# BS-P5-CONCEPT-E2E-BLACK-BOX · E2E tests treat the running application as a black box across browser, API and persistence boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-E2E-BLACK-BOX**

## Concept

E2E tests treat the running application as a black box across browser, API and persistence boundaries

## Prerequisites

- BS-P5-CONCEPT-FRONTEND-INTEGRATION-BOUNDARY
- BS-FSO-3.1

## BodySense target files

- `apps/web/project.json`
- `playwright.config.ts`

## Prediction before reading/running

Choose one BodySense Playwright spec and predict every real process/boundary it expects: browser, Vite, API, auth/state, persistence/external services as applicable.

## Task

Choose a BodySense E2E flow and enumerate all real layers traversed. Explain why a failure has wider confidence but harder fault localization than a component test.

## Failure case

Call internal functions directly in an E2E test or mock away the API/persistence boundary, then claim full-stack confidence.

## Verification command / evidence

- Inspect `playwright.config.ts`, `apps/web/project.json` and one `apps/web/e2e/*.spec.ts`.
- Run `pnpm exec playwright test --list` to verify test discovery/config without requiring the full external environment; execute a real spec only when its API/data prerequisites are available.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes E2E black-box?
- What can a passing E2E prove that component tests cannot?
- Why does an E2E failure need narrower tests/traces for diagnosis?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
