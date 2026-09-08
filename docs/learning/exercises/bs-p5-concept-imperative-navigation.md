# BS-P5-CONCEPT-IMPERATIVE-NAVIGATION · imperative navigation is appropriate after events such as successful mutation/logout, while links remain declarative navigation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-IMPERATIVE-NAVIGATION**

## Concept

imperative navigation is appropriate after events such as successful mutation/logout, while links remain declarative navigation

## Prerequisites

- BS-P5-CONCEPT-CLIENT-ROUTING
- BS-P1-CONCEPT-EVENT-HANDLING

## BodySense target files

- `apps/web/src/App.tsx`
- `apps/web/src/features`

## Prediction before reading/running

For successful login/register, predict when navigation should happen, what should happen on failure, and why navigation is triggered by the event outcome rather than render.

## Task

Find or design a BodySense post-action navigation and explain why useNavigate is used there rather than rendering a Link.

## Failure case

Navigate during render or before the mutation succeeds, producing loops or moving the user into a protected route after failed auth.

## Verification command / evidence

- Trace `useNavigate` in `LoginForm.tsx` and `RegisterForm.tsx`.
- Use focused store/component reasoning plus route tests to prove success and failure paths have different navigation authority; add a small test only if the path is not characterized.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- When is imperative navigation appropriate?
- When should a Link/Navigate element be preferred?
- Why must mutation success own post-action navigation timing?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
