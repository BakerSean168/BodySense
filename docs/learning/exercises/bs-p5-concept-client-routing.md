# BS-P5-CONCEPT-CLIENT-ROUTING · client-side routing maps URL state to React views without full document navigation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source concept: **FSO-P5-CONCEPT-CLIENT-ROUTING**

## Concept

client-side routing maps URL state to React views without full document navigation

## Prerequisites

- BS-P1-CONCEPT-COMPONENT
- BS-FSO-0.5

## BodySense target files

- `apps/web/src/App.tsx`
- `apps/web/src/main.tsx`

## Prediction before reading/running

Given `/consultation/abc`, predict which React route element renders, whether the browser document reloads, and which history entry changes before reading App.tsx.

## Task

Trace one BodySense route from URL to React Router route element and compare link/navigation behavior with a full browser document request.

## Failure case

Use a full document navigation for an internal SPA transition and lose in-memory UI/cache state unnecessarily, or let catch-all routing hide a genuinely invalid route without intentional UX.

## Verification command / evidence

- Trace `BrowserRouter -> Routes -> Route` in `apps/web/src/App.tsx` and the root mount in `main.tsx`.
- Run `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/pages/__tests__/ConsultationPage.test.tsx` and identify how MemoryRouter/Routes model the client route.

Passing existing build/tests is **not** sufficient for L4. The learner must explain which behavior/architecture invariant the evidence discriminates, what is outside the evidence, and what observation would falsify the conclusion.

## Explain-back questions

- What state does the URL own?
- What is the difference between SPA navigation and a document request?
- What does back/forward navigation mean for React state and server data?

## Production change

No production change is required when the current design already satisfies the concept. If a real gap is exposed, characterize it with the smallest focused test/trace/measurement before making the minimal architecture-consistent correction.

## L4 acceptance

Complete only when the learner can make the prediction before inspecting the answer path, verify or falsify it with focused evidence, explain the failure mode and independently justify the relevant routing/testing/build/ownership trade-off.
