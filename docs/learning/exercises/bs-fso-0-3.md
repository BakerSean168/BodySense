# BS-FSO-0.3 · form controls, submission, validation and request payload

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-0.3 · HTML forms**

## Concept

form controls, submission, validation and request payload

## Prerequisites

- FSO-0.1

## BodySense target files

- `apps/web/src/features/auth/components/LoginForm.tsx`
- `apps/web/src/features/auth/services/authService.ts`
- `apps/api/internal/transport/httpapi/auth.go`

## Prediction before reading/running

Predict which values are browser/local state, which event prevents default navigation, what JSON payload is sent, and what the browser would do if the submit handler did not prevent the native form submission.

## Task

Trace the real login form from label/input -> React state -> submit event -> auth service -> HTTP request. Compare native form semantics with React-controlled behavior.

## Failure case

Model an invalid email/password submission and a network/auth failure. Distinguish client-side validation from server-side authentication and identify what must still be enforced server-side.

## Verification command / evidence

- `pnpm exec vitest run --config apps/web/vite.config.ts` with the relevant auth/form tests if present
- `cd apps/api && go test ./internal/handler ./internal/service -run Auth -count=1`
- Optional DevTools Network trace of a development login request.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why is client validation not a security boundary?
- What does `preventDefault` change?
- Which values should never be logged from this form flow?

## Production change

none unless a validation/semantic/security gap is demonstrated.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
