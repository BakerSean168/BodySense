# BS-FSO-0.5 · SPA navigation, initial render and server-state hydration

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-0.5 · Single page app diagram**

## Concept

SPA navigation, initial render and server-state hydration

## Prerequisites

- FSO-0.4

## BodySense target files

- `apps/web/src/features/consultation/pages/ConsultationPage.tsx`
- `apps/web/src/features/workspace/hooks/useHealthWorkspaceQuery.ts`
- `apps/web/src/features/workspace/api/workspaceQueryOptions.ts`

## Prediction before reading/running

Predict the order among route match, component render, query subscription, network request, loading projection, successful cache write, and rerender.

## Task

Trace initial navigation to a BodySense workbench route and draw the SPA sequence from URL/router to React rendering and TanStack Query hydration. Mark which parts can render before server data arrives.

## Failure case

Model a slow or failed workspace request. Predict which UI remains available, which error/loading state appears, and whether the route identity itself is lost.

## Verification command / evidence

- `DevTools Network + React observation on a development route, or a focused query/render test`
- `Inspect the query options and the component branch that consumes loading/error/data.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is SPA navigation not equivalent to a full HTML document reload?
- Who owns the URL identity?
- Why can React render before the server response exists?

## Production change

none unless loading/error/recovery behavior is demonstrably broken.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
