# BS-P5-CONCEPT-ROUTE-PARAMS · parameterized client routes bind URL path segments to resource/view identity

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-ROUTE-PARAMS**

## Concept

parameterized client routes bind URL path segments to resource/view identity

## Prerequisites

- BS-P5-CONCEPT-CLIENT-ROUTING

## BodySense target files

- `apps/web/src/App.tsx`
- `apps/web/src/features`

## Prediction before reading/running

For `/consultation/:id` and `/consultation/share/:token`, predict the extracted parameter, its consumer, and behavior for missing/malformed identity before reading the page code.

## Task

Trace a BodySense route parameter into the page/query that loads its resource. Predict malformed/missing IDs and back/forward navigation behavior.

## Failure case

Treat a route parameter as trusted domain identity and issue data access without validation/ownership checks, or derive detail state from an unrelated collection that is absent on deep link.

## Verification command / evidence

- Trace `useParams` in `ConsultationPage.tsx` and `SharePage.tsx` from URL identity to the query/service boundary.
- Run the consultation page and share page focused tests and identify the deep-link/parameter contract.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- Who validates route parameter syntax and ownership?
- Why must deep links work without prior list navigation?
- How is a route param different from query/search state?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
