# BS-P5-CONCEPT-FORM-LABELS · form labels provide semantic name, click target and accessible control association

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-FORM-LABELS**

## Concept

form labels provide semantic name, click target and accessible control association

## Prerequisites

- BS-P2-CONCEPT-CONTROLLED-COMPONENT

## BodySense target files

- `apps/web/src/features/auth/components/LoginForm.tsx`
- `apps/web/src/features/auth/components/RegisterForm.tsx`
- `apps/web/src/components/ui/Input.tsx`

## Prediction before reading/running

For each auth input, predict its accessible name and the exact label-control association, then predict what a screen reader/Testing Library role query would see.

## Task

Inspect BodySense auth fields for explicit/implicit label associations and predict accessible names used by users, screen readers and Testing Library queries.

## Failure case

Render visually adjacent text without `htmlFor`/wrapped association so clicking the label and accessible-name lookup no longer target the input.

## Verification command / evidence

- Trace `label` + `id` through `LoginForm.tsx`/`RegisterForm.tsx` into `components/ui/Input.tsx`.
- Use an isolated Testing Library render or browser accessibility inspection to verify inputs are discoverable by role/name.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What does `<label for>` add beyond visual text?
- How do id uniqueness and reusable input wrappers interact?
- Why do semantic labels improve both accessibility and tests?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
