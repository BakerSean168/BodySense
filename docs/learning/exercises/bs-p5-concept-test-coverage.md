# BS-P5-CONCEPT-TEST-COVERAGE · coverage measures executed code, not correctness or behavioral completeness

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-TEST-COVERAGE**

## Concept

coverage measures executed code, not correctness or behavioral completeness

## Prerequisites

- BS-P5-CONCEPT-COMPONENT-TEST-RENDER

## BodySense target files

- `apps/web/vite.config.ts`
- `package.json`

## Prediction before reading/running

Predict one branch/function that coverage can report as executed while a meaningful behavioral invariant is still wrong.

## Task

Explain statement/branch/function/line coverage and identify a BodySense behavior that could remain wrong despite high coverage. Use coverage only as gap evidence, never a quality proof.

## Failure case

Use a high line/statement percentage as a correctness target and add low-value tests that execute code without discriminating failures.

## Verification command / evidence

- Inspect coverage configuration/scripts in `vite.config.ts` and root `package.json`.
- Run a focused coverage command when practical and map one uncovered line/branch to risk; also name one high-coverage behavior that still needs a semantic assertion.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What do statement/branch/function/line coverage measure?
- Why is 100% coverage not 100% correctness?
- When is coverage useful as gap evidence?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
