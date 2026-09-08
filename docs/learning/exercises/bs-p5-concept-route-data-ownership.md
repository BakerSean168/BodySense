# BS-P5-CONCEPT-ROUTE-DATA-OWNERSHIP · route detail views should own resource-specific loading instead of receiving unrelated collection state

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-ROUTE-DATA-OWNERSHIP**

## Concept

route detail views should own resource-specific loading instead of receiving unrelated collection state

## Prerequisites

- BS-P5-CONCEPT-ROUTE-PARAMS
- BS-P6-CONCEPT-TANSTACK-QUERY

## BodySense target files

- `apps/web/src/features`
- `apps/web/src/App.tsx`

## Prediction before reading/running

Open a detail route directly in a fresh browser session and predict which route identity/query owns loading rather than relying on an in-memory parent collection.

## Task

Compare passing a whole collection into a detail view with querying by route identity. Explain cache, reload/deep-link and ownership consequences in BodySense.

## Failure case

Pass a previously loaded collection as the only source of detail data so reload/deep-link fails or stale list state becomes canonical.

## Verification command / evidence

- Trace `ConsultationPage.tsx` route id/search params into consultation/query hooks and compare with a hypothetical parent-passed collection.
- Use a route/page test to identify what can be reconstructed from URL + server state after reload.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What makes a route self-sufficient?
- Which state belongs in URL, query cache, local UI state, or durable server state?
- Why is parent collection state a fragile detail-page dependency?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
