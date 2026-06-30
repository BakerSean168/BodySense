# Phase 01a: ContextBuilder Module

## Goal

Create a Go `ContextBuilder` Module that owns chat context assembly for the Python AI service, without changing current chat behavior.

## Why

`docs/architecture/system-engineering-refactor-plan.md` identifies `ChatHandler.SendMessage` as too shallow: request parsing, message persistence, context assembly, SSE mapping, and run state handling live in one implementation. This ticket creates the first seam by moving profile/history/consultation context assembly behind a small interface.

## Current State

- `apps/api/internal/handler/chat_handler.go` builds AI context inline after creating the run.
- It loads profile through `ProfileService.GetProfile`.
- It loads consultation phase and `extracted_info` through `ConsultationService.GetConsultation`.
- It loads conversation history through `MessageService.GetMessages`, filters to previous-turn completed messages, and converts messages to `service.ChatMessage`.
- `apps/api/internal/service/ai_client.go` defines `ChatStreamRequest` and `ChatMessage`.
- There is no dedicated context trace, context budget, or reusable context assembly Module.

## Scope

### Allowed

- Add a Go context assembly Module for chat turns.
- Move existing inline context-building logic from `ChatHandler.SendMessage` into that Module.
- Preserve the current `service.ChatStreamRequest` shape.
- Add focused unit tests for context filtering rules if current constructors make this practical.
- Add small DTO/types needed for the Module interface.

### Not Allowed

- Do not introduce ToolRuntime, JobRuntime, ask_user, or resume.
- Do not change Python request handling.
- Do not change prompt content or context truncation semantics beyond existing `completed previous turns only`.
- Do not add migrations.
- Do not change SSE event names.
- Do not modify `docs/architecture/README.md`.

## Target Files

- `apps/api/internal/context/context_builder.go` (new, likely)
- `apps/api/internal/context/context_builder_test.go` (new, likely)
- `apps/api/internal/handler/chat_handler.go`
- `apps/api/internal/service/ai_client.go` (only if a narrow type move or comment is needed)
- `apps/api/internal/service/service_interfaces.go` or existing interface file (likely, only if needed)

## Design Notes

Suggested interface:

```go
type Builder interface {
    BuildChatContext(ctx context.Context, input BuildChatContextInput) (*service.ChatStreamRequest, *ContextTrace, error)
}
```

`BuildChatContextInput` should include `ConversationID`, `TurnID`, `UserID`, `RequestContext`, `MessageParts`, `IsDraft`, and `Entry`.

`ContextTrace` should be lightweight and safe to persist later:

```go
type ContextTrace struct {
    IncludedMessageIDs []uuid.UUID
    ExcludedCurrentTurn bool
    ProfileIncluded bool
    ConsultationIncluded bool
    UseCase string
}
```

For this ticket, `ContextTrace` may stay in memory or be written into `run.metadata` only if that can be done without widening the change. The main goal is the seam and behavior preservation.

## Implementation Steps

1. Create the `apps/api/internal/context/` package.
2. Define `BuildChatContextInput`, `ContextTrace`, and `Builder`.
3. Implement `ContextBuilder` using the existing services currently called by `ChatHandler`.
4. Move the existing text extraction from message parts into the builder implementation.
5. Move current profile JSON fallback behavior into the builder implementation: missing profile should become `{}`.
6. Move current consultation fallback behavior into the builder implementation: missing extracted info should become `[]`, missing phase should become empty string.
7. Move current history filtering into the builder implementation: exclude current turn and include only `Status == "completed"` messages with non-empty text.
8. Update `ChatHandler` construction to receive the builder, or instantiate it from existing dependencies if the local dependency injection shape makes that smaller.
9. Replace the inline context assembly block in `SendMessage` with `builder.BuildChatContext(...)`.
10. Add unit tests for history filtering and fallback JSON behavior if the existing service interfaces allow simple fakes.

## Invariants

- The outbound Python request remains compatible with `/api/chat/stream`.
- The user-visible chat response remains unchanged.
- Current turn user message must not be duplicated in `Messages`.
- Streaming, failed, and aborted assistant messages must not enter model context.
- New conversations still create `consultation_sessions` when entry is `consultation`.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
```

If Nx is unavailable in the execution environment:

```bash
cd apps/api
go vet ./...
go test ./...
```

## Acceptance Criteria

- [ ] `ChatHandler.SendMessage` no longer contains profile/history/consultation context assembly logic.
- [ ] A dedicated `ContextBuilder` Module builds the same `service.ChatStreamRequest` shape as before.
- [ ] Previous-turn completed-message filtering is covered by tests or explicitly documented as manually verified.
- [ ] No ToolRuntime, JobRuntime, ask_user, migration, or frontend changes are included.
- [ ] Existing chat SSE behavior remains unchanged.

## Regression Risks

- Accidentally including the current user message twice in model context.
- Dropping profile or consultation phase when the session exists.
- Including streaming or failed assistant messages in context.
- Dependency injection churn causing unrelated handler initialization failures.

## Out of Scope Follow-ups

- Context token budgeting.
- Long-term memory summaries.
- `consultation_state` and `state_revision`.
- Context trace persistence and replay UI.

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

