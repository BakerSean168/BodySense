# BS-FSO-3.1 · serve a collection/read model through Gin REST

> Status: exercise-ready
> Required mastery: **L4 Verify**
> Source point: **FSO-3.1 · collection backend**

## Concept

serve a collection/read model through Gin REST

## Prerequisites

- FSO-0.4

## BodySense target files

- `apps/api/cmd/server/main.go`
- `apps/api/internal/handler/health_workspace_handler.go`
- `apps/api/internal/service/health_workspace_service.go`

## Prediction before reading/running

Before reading route registration, predict the method/path, auth requirement, handler signature, success status and response owner for the HealthWorkspace read model.

## Task

Trace one real BodySense collection/read-model endpoint from route registration through handler/service to underlying repositories/services. Separate route wiring, transport response, projection composition and persistence ownership.

## Failure case

Model missing auth and a downstream repository failure. Predict which layer decides 401 versus 500 and what public detail is safe to return.

## Verification command / evidence

- `cd apps/api && go test ./internal/handler ./internal/service -run "Workspace|HealthWorkspace" -count=1`
- Optional `curl` against local dev for a non-destructive GET.

Passing existing tests is **not** enough for L4. The learner must explain why the selected test/trace proves the predicted invariant and identify what observation would falsify the prediction.

## Explain-back questions

- What makes an endpoint REST-like here?
- Why should a handler not build durable state itself?
- `What is the difference between a domain aggregate and a read-model/projection endpoint?`

## Production change

none unless the trace exposes inconsistent transport/domain responsibility.

## L4 acceptance

This card is complete only when the learner can independently design or select a test/trace that distinguishes correct behavior from the failure case, run or inspect the evidence, and explain the result without relying on an AI-generated conclusion.
