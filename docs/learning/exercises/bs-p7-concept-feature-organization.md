# BS-P7-CONCEPT-FEATURE-ORGANIZATION · frontend code organization should follow change/ownership boundaries rather than arbitrary file-type buckets

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P7-CONCEPT-FEATURE-ORGANIZATION**

## Concept

frontend code organization should follow change/ownership boundaries rather than arbitrary file-type buckets

## Prerequisites

- BS-P1-CONCEPT-COMPONENT

## BodySense target files

- `apps/web/src/features`
- `apps/web/src/components`
- `apps/web/src/stores`

## Prediction before reading/running

Given a new consultation-specific hook and a generic button, predict where each belongs and what dependency direction should prevent feature leakage.

## Task

Review BodySense feature-based structure and classify one cross-cutting component/store versus feature-local API/hook/component. Explain where a new feature file should live and why.

## Failure case

Create global file-type buckets for every component/hook/service so changes span unrelated domains, or place feature-specific code in shared folders and invite reverse dependencies.

## Verification command / evidence

- Map `apps/web/src/features`, `components`, and `stores` into feature-local versus cross-cutting ownership.
- Choose three files and explain why moving each across the feature/shared boundary would improve or worsen coupling.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes code feature-local versus shared?
- Why should shared code have a higher reuse/stability bar?
- How does organization affect dependency direction and change locality?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
