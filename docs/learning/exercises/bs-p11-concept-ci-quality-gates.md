# BS-P11-CONCEPT-CI-QUALITY-GATES · lint, typecheck, tests and builds form independent automated gates whose failures block unsafe promotion

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P11-CONCEPT-CI-QUALITY-GATES**

## Concept

lint, typecheck, tests and builds form independent automated gates whose failures block unsafe promotion.

## Prerequisites

- BS-P11-CONCEPT-REPRODUCIBLE-PIPELINE

## BodySense target files

- `.github/workflows/ci.yml`
- `package.json`

## Prediction before reading/running

Map lint/typecheck/unit/integration/build/security/delivery checks to the failure class each is intended to block and the downstream jobs they gate.

## Task

Map BodySense quality checks to failure classes. Verify that a failure returns nonzero and blocks downstream release/deploy rather than becoming a warning.

## Failure case

Mark a critical test as continue-on-error or run deploy independently of required checks. Predict how a red candidate can reach production.

## Verification command / evidence

- Inspect `.github/workflows/ci.yml`/release workflows and draw the `needs`/required-check graph.
- `pnpm curriculum:check && pnpm test:delivery` as examples of repository gates.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why do multiple gates exist instead of one “test” command?
- Which checks must block merge vs deploy?
- How do flaky gates damage trust?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
