# BS-A7 · streaming event ledger, disconnect/cancel, interrupt/resume, replay and deduplication

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **BodySense Agent Extension A7**

## Concept

streaming event ledger, disconnect/cancel, interrupt/resume, replay and deduplication

## Prerequisites

- BS-A1
- BS-A2

## BodySense target files

- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts`
- `apps/web/src/features/consultation/runtime/activeTurnReducer.ts`
- `apps/web/src/features/consultation/runtime/durableRunRecovery.ts`
- `apps/api/internal/service/consultation_service.go`
- `apps/api/internal/repository/runtime_event_repository.go`

## Prediction before reading/running

Draw two timelines before reading tests: (1) browser transport disconnect while execution continues, then recovery; (2) ask_user interrupt, durable answer, and LangGraph resume. Predict which IDs and sequence numbers remain stable.

## Task

Trace public event creation/persistence, `(run_id, seq)` ownership, parser/reducer deduplication, recovery, explicit cancellation, and interrupt resume. Separate transport state, Go durable ledger, and Python checkpoint state.

## Failure case

Simulate or reason about duplicate replay events, a reconnect after partial delivery, and a transport abort confused with business cancellation. Identify the first invariant that prevents double-application or accidental cancellation.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/consultation/runtime/activeTurnReducer.test.ts apps/web/src/features/consultation/runtime/durableRunRecovery.test.ts apps/web/src/features/consultation/hooks/useSSEProcessor.test.ts`
- `cd apps/api && go test ./internal/service ./internal/repository -run "Consultation|RuntimeEvent" -count=1`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Who owns public sequence numbers?
- Why is disconnect not cancellation?
- Why does a resumed logical thread receive a new run sequence domain?
- What makes replay idempotent in the browser?

## Production change

none unless a replay/cancel/interrupt invariant is not already characterized; add a focused regression test before changing runtime behavior.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
