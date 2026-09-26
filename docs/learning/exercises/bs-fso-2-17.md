# BS-FSO-2.17 · stale-client mutation failure under concurrent server changes

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-2.17 · Phonebook step 12**

## Concept

stale-client mutation failure under concurrent server changes

## Prerequisites

- FSO-2.11
- FSO-0.6

## BodySense target files

- `apps/web/src/features/workspace/hooks/useBodyStateCommand.ts`
- `apps/web/src/lib/api-client.ts`
- `apps/api/internal/service/body_state_service.go`
- `apps/api/internal/transport/httpapi/body_state.go`

## Prediction before reading/running

Model two tabs A/B starting from the same BodyState revision. Predict the revision after A commits, B's response when it writes the stale revision, and B's next read.

## Task

Use an existing BodyState command as the domain equivalent of the stale Phonebook update. Build a two-client timeline and then verify that conflict handling preserves server truth and refreshes the client projection rather than silently overwriting it.

## Failure case

Remove/skip the expected-revision check conceptually and describe the lost-update class of bug that becomes possible. Then remove/skip client invalidation conceptually and describe the stale-projection bug.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts apps/web/src/features/workspace/hooks/useBodyStateCommand.test.tsx`
- `cd apps/api && go test ./internal/service -run BodyState -count=1`
- If existing tests do not exercise a two-client conflict, add a focused characterization test before any production change.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What does HTTP 409 mean in this BodySense flow?
- Which correctness guarantee comes from optimistic concurrency and which comes from client cache invalidation?
- Why is retrying the exact stale mutation blindly unsafe?

## Production change

test addition is encouraged if the conflict path is not already characterized; production behavior changes only for a proven defect.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
