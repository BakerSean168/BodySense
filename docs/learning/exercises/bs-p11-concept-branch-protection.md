# BS-P11-CONCEPT-BRANCH-PROTECTION · branch protection makes required checks/reviews an enforced merge invariant rather than team convention

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P11-CONCEPT-BRANCH-PROTECTION**

## Concept

branch protection makes required checks/reviews an enforced merge invariant rather than team convention.

## Prerequisites

- BS-P11-CONCEPT-CI-QUALITY-GATES

## BodySense target files

- `.github/workflows/repository-governance.yml`
- `docs/issue-workflow.md`

## Prediction before reading/running

List checks/reviews that must be impossible to bypass for normal contributors before main changes.

## Task

Enumerate BodySense main protection requirements and bypass authority. Explain why CI that can be ignored does not keep main green.

## Failure case

CI exists but repository permits direct push/merge while required checks are red. Explain why “we usually wait for CI” is not a control.

## Verification command / evidence

- Inspect `.github/workflows/repository-governance.yml` and `docs/issue-workflow.md`.
- Record enforced versus documented-only controls and any explicit admin/break-glass bypass.

Passing an existing test is **not** sufficient for L4. The learner must explain which invariant the evidence proves, which relevant layer is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Why does CI need repository enforcement?
- Which branch protection rules can deadlock automation?
- How should break-glass authority be audited?

## Production change

No production change is required when the current design already satisfies the source concept. If the exercise exposes a real gap, first add the smallest characterization/regression evidence that demonstrates the gap, then make the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify behavior with focused code/test/runtime evidence, explain the failure case and independently justify the ownership/contract trade-off.
