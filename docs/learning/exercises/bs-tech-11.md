# BS-TECH-11 · REST transport semantics and handler/service/repository separation

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **TECH SCHOOL Lecture #11 · RESTful HTTP API in Gin**

## Concept

REST transport semantics and handler/service/repository separation

## Prerequisites

- TECH-01
- FSO-3.1

## BodySense target files

- `apps/api/cmd/server/main.go`
- `apps/api/internal/handler/body_state_handler.go`
- `apps/api/internal/service/body_state_service.go`
- `apps/api/internal/repository/body_state_repository.go`

## Prediction before reading/running

For one BodyState endpoint, predict route, method, authentication requirement, request DTO, success status, and one validation/conflict status before reading the handler.

## Task

Trace one GET and one mutation from route registration through handler -> service -> repository. Mark which concerns belong to transport, business policy, and persistence.

## Failure case

Send/model malformed JSON, invalid ID, missing auth, stale revision, and internal DB failure. Predict the public status/code for each and identify the first responsible layer.

## Verification command / evidence

- `cd apps/api && go test ./internal/handler ./internal/service -run BodyState -count=1`
- `Optional local `curl`/browser trace against a disposable/dev user.`

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- Why should a handler not own domain concurrency rules?
- When is 400 different from 409?
- What information must never leak from a 500 response?

## Production change

none unless the trace exposes mixed responsibilities or inconsistent public semantics.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
