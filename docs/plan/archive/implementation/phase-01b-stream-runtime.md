# Phase 01b: StreamRuntime Module

> **⚠️ SUPERSEDED — 本方案的实施路径已被 ADR 0002 取代。**
>
> 本 Phase 设计的从 `ChatHandler` 抽取 `StreamRuntime` 的路径不再适用（`ChatHandler` 已删除）。StreamEvent 契约设计仍然有效，但事件产出点已迁移到 consultation thread runtime。详见 [`docs/adr/0002-agent-runtime-ownership.md`](../../../adr/0002-agent-runtime-ownership.md)。

## Goal

Create a Go `StreamRuntime` Module that owns StreamEvent validation, ID enrichment, sequence assignment, and SSE writing while preserving the current event contract.

## Why

The system refactor plan requires an explicit SSE event contract seam before ToolRuntime, ask_user, and JobRuntime are introduced. Today `ChatHandler` maps Python events, assigns outbound `seq`, fills IDs, writes SSE frames, and applies some domain side effects inline. This ticket concentrates stream mechanics in one Module for Locality.

## Current State

- `apps/api/internal/handler/sse_writer.go` writes SSE frames and sets headers.
- `apps/api/internal/dto/stream_event.go` defines `StreamEvent`, `StreamEventIDs`, and `NewStreamEvent`.
- `apps/api/internal/handler/chat_handler.go` has `writeStreamEvent` and `prepareOutboundEvent`.
- `ChatHandler.SendMessage` switches on Python event types and directly calls `sse.WriteEvent`.
- `packages/contracts/src/stream-events.ts` and `packages/contracts/schemas/stream-event.v1.schema.json` define the client-facing contract, but Go does not validate event names beyond struct shape.

## Scope

### Allowed

- Add a Go stream Module around existing `SSEWriter`.
- Move `writeStreamEvent` and `prepareOutboundEvent` behavior into the new Module.
- Keep current event names, channels, payloads, and ordering.
- Add tests for sequence increments, ID defaults, empty payload defaults, and passthrough event behavior.
- Optionally add a small known-event allowlist in warn-only mode if it does not reject current events.

### Not Allowed

- Do not add `stream_events` table or persistence in this ticket.
- Do not introduce new event names like `tool.call.created`.
- Do not add `job_id` or `interaction_id` yet unless needed only as nullable fields in a later ticket.
- Do not change frontend SSE consumption.
- Do not change Python event emission.
- Do not implement replay.

## Target Files

- `apps/api/internal/stream/runtime.go` (new, likely)
- `apps/api/internal/stream/runtime_test.go` (new, likely)
- `apps/api/internal/handler/chat_handler.go`
- `apps/api/internal/handler/sse_writer.go` (likely, only if moving constructor helpers)
- `apps/api/internal/dto/stream_event.go` (likely, only if shared helper signatures need small changes)

## Design Notes

Suggested interface:

```go
type Runtime interface {
    NewWriter(w http.ResponseWriter, base StreamBaseIDs) StreamWriter
}

type StreamWriter interface {
    Send(ctx context.Context, event dto.StreamEvent, opts SendOptions) error
    SendNew(ctx context.Context, channel string, eventType string, ids dto.StreamEventIDs, payload any) error
}
```

For this ticket, `StreamRuntime` is an adapter over the existing `SSEWriter`. It should not know consultation business rules. Domain side effects such as phase update remain in `ChatHandler` until later tickets create a deeper application Module.

## Implementation Steps

1. Create `apps/api/internal/stream/`.
2. Define a `Runtime` or `Writer` interface that hides outbound sequence allocation.
3. Implement ID enrichment equivalent to current `prepareOutboundEvent`.
4. Implement event creation equivalent to current `writeStreamEvent`.
5. Update `ChatHandler.SendMessage` to construct/use the new stream writer after run creation.
6. Replace direct calls to `h.writeStreamEvent` and `h.prepareOutboundEvent` with `StreamRuntime` calls.
7. Keep `SSEWriter` as the low-level SSE adapter or move it only if imports remain clean.
8. Add tests for:
   - `seq` starts at 1 and increments.
   - base conversation/run/turn IDs are applied when missing.
   - message ID is applied for message-scoped events.
   - empty payload becomes `{}`.
9. Remove obsolete private helper methods from `ChatHandler` after all call sites are moved.

## Invariants

- SSE frame format remains `event: <type>` and `data: <StreamEvent JSON>`.
- Existing frontend continues to receive the same event names.
- `conversation.created`, `message.persisted`, and `message.created` remain first-turn events in the same order.
- `stream.done` still terminates successful and handled-error streams.
- `maxSSEEvents` and `sseTimeout` behavior remains unchanged.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
```

## Acceptance Criteria

- [ ] `ChatHandler` no longer owns sequence allocation or ID enrichment helper methods.
- [ ] `StreamRuntime` tests cover the current stream mechanics.
- [ ] No new database table or replay behavior is added.
- [ ] Current StreamEvent payloads remain compatible with `packages/contracts`.
- [ ] Chat streaming still compiles and tests pass.

## Regression Risks

- Off-by-one sequence changes causing frontend ordering bugs.
- Missing `message_id` on assistant text events.
- Accidentally double-marshalling payload JSON.
- Panic behavior from `SSEWriter` changing unexpectedly.

## Out of Scope Follow-ups

- `stream_events` persistence.
- Replay after reconnect.
- Heartbeat events.
- New tool/job/interaction channels and IDs.

## Final Response Format for Coding Agent

```md
Changed files:
- ...

Behavior changes:
- ...

Tests run:
- ...

Tests passed / failed:
- ...

Known risks:
- ...

Follow-up tasks:
- ...
```

