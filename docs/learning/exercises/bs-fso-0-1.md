# BS-FSO-0.1 · semantic HTML and rendered DOM structure

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-0.1 · HTML**

## Concept

semantic HTML and rendered DOM structure

## Prerequisites

- None

## BodySense target files

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/workspace/components/BodyStateWorkbench.tsx`

## Prediction before reading/running

Before opening DevTools, predict the major page landmarks, heading hierarchy, interactive controls, and which elements should have native semantics instead of generic divs.

## Task

Open one real BodySense route and correlate JSX with the rendered DOM. Identify landmark elements, headings, labels, buttons/links, and one accessibility relationship. Do not redesign the page unless a semantic defect is demonstrated.

## Failure case

Pick one interactive element and imagine it implemented as a non-semantic clickable div. State what keyboard/accessibility behavior would be lost and how a browser/a11y check would reveal it.

## Verification command / evidence

- `Browser Elements/accessibility tree inspection on the chosen route`
- `Optional focused Testing Library/Playwright assertion for semantic role/name when a gap is suspected.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why does semantic HTML matter even when Tailwind can make any element look correct?
- Which semantics come from native HTML and which require ARIA?
- Why should ARIA not replace an available native element?

## Production change

none by default; a demonstrated semantic/accessibility defect can justify a focused fix/test.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
