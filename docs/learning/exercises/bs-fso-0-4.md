# BS-FSO-0.4 · HTTP mutation sequence and responsibility boundaries

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-0.4 · New note diagram**

## Concept

HTTP mutation sequence and responsibility boundaries

## Prerequisites

- FSO-0.1
- FSO-0.3

## BodySense target files

- `apps/web/src/features/workspace/api/workspaceApi.ts`
- `apps/api/internal/transport/httpapi/body_state.go`
- `apps/api/internal/service/body_state_service.go`
- `apps/api/internal/repository/body_state_repository.go`

## Prediction before reading/running

Before tracing code, write the expected method/path/payload, where authentication is checked, where `expected_revision` is enforced, what is committed, and which HTTP status you expect for success and stale input.

## Task

Use `workspaceApi.addFact` as the concrete BodySense mutation. Draw the full sequence browser -> auth fetch -> Gin route/handler -> service -> repository/PostgreSQL -> response. Annotate ownership at each hop. Then compare the drawing with executable code.

## Failure case

Repeat the sequence on paper with a stale `expected_revision`. Predict the first layer that rejects it, the public status/code, and whether any durable state may change.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/workspace/hooks/useBodyStateCommand.test.tsx`
- `cd apps/api && go test ./internal/handler ./internal/service -run BodyState -count=1`
- One DevTools Network trace is optional but preferred when the local stack is running.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is CORS not part of this business mutation sequence?
- Which layer owns durable BodyState truth?
- Why is the browser allowed to send an expected revision but not to decide whether the revision is current?

## Production change

none by default; only a demonstrated contract/observability gap justifies production change.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
