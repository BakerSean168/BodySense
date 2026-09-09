# BodySense contract-codegen spike common fixture

This directory is the shared baseline for all candidate worktrees created from `spike/contract-codegen-common`.

## Scope

- R1: `GET /api/v1/health-workspace`
- R2 primary: `POST /api/v1/body-state/facts`
- R2 path-param control: `PATCH /api/v1/body-state/facts/{fact_id}/review`
- S1 and I1 are evaluated in the JSON Schema and Proto candidate worktrees.

The three R2 candidates named in the plan cannot simultaneously satisfy the plan's stated path/query + optimistic-revision criteria. `addFact` is therefore the primary R2 sample because it exercises a real request body, optimistic revision and 409 semantics; `reviewFact` is retained only as a path-parameter control.

## Rules

1. Candidate worktrees must start from the same common commit.
2. OpenAPI candidates must use `spec/bodysense-spike.openapi.yaml` unchanged for the first comparison pass.
3. Generated output is isolated under each candidate's `experiments/contract-codegen/<candidate>/gen/` directory.
4. Production routes, handlers, transports and dependencies are not modified by the spike.
5. Candidate-specific workarounds must be recorded rather than silently folded into the common spec.

## Baseline source anchors

- `apps/api/internal/dto/health_workspace.go`
- `apps/api/internal/dto/body_state.go`
- `apps/api/internal/handler/body_state_handler.go`
- `apps/web/src/features/workspace/types/workspace.ts`
- `apps/web/src/features/workspace/api/workspaceApi.ts`
- `packages/contracts/src/stream-events.ts`
- `packages/contracts/src/stream-event-parser.ts`
- `packages/contracts/schemas/stream-event.v1.schema.json`
- `apps/api/internal/service/ai_client.go`
- `apps/ai-service/src/api/routes/runtime.py`

## Baseline observation

The current handwritten web `request<T>()`/`expectJson<T>()` path gives a compile-time view but does not establish runtime response validation. The spike treats network payloads as untrusted until a candidate demonstrates an actual runtime parse/validation step.
