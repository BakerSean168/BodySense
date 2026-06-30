# Phase 02d: agent_tool_calls Audit Persistence

## Goal

Add Go-side `agent_tool_calls` persistence and record current tool call lifecycle events without implementing HITL or resume.

## Why

ToolRuntime is not production-grade until tool calls are auditable. The system needs to answer: which tool did the model request, with what arguments, for which run, and what status/result/error occurred. This ticket adds audit Locality while keeping existing synchronous tool behavior.

## Current State

- Go has `runs`, `messages`, and `conversations` from `000013_session_redesign.up.sql`.
- There is no `agent_tool_calls` table or model.
- Python emits current tool events as `tool.call` and `tool.result`.
- Go `ChatHandler.SendMessage` currently forwards `tool.call` and `tool.result`, and appends them to assistant message parts.
- `dto.StreamEventIDs` has `tool_call_id` but no DB model uses it.

## Scope

### Allowed

- Add migration for `agent_tool_calls`.
- Add Go model, repository, and small service for audit persistence.
- Persist `tool.call` as created/running or pending.
- Persist `tool.result` as succeeded or failed depending on payload if possible.
- Preserve existing SSE behavior and message parts.
- Include idempotency around `(run_id, tool_call_id)` or an equivalent unique key.

### Not Allowed

- Do not add `agent_interactions`.
- Do not implement ask_user, waiting_user, or resume.
- Do not rename current stream events.
- Do not require Python to call a new audit endpoint.
- Do not block user-visible streaming if audit persistence fails; log and continue unless a DB transaction is already required.

## Target Files

- `apps/api/migrations/000015_create_agent_tool_calls.up.sql` (new, likely)
- `apps/api/migrations/000015_create_agent_tool_calls.down.sql` (new, likely)
- `apps/api/internal/model/agent_tool_call.go` (new, likely)
- `apps/api/internal/repository/agent_tool_call_repository.go` (new, likely)
- `apps/api/internal/service/agent_tool_service.go` (new, likely)
- `apps/api/internal/handler/chat_handler.go`
- `apps/api/internal/dto/stream_event.go` (likely only if helper methods are useful)

## Design Notes

Suggested table:

```sql
agent_tool_calls (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
  tool_call_id TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  arguments JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(30) NOT NULL,
  result JSONB,
  error JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}',
  UNIQUE (run_id, tool_call_id)
)
```

Current status mapping:

- `tool.call` -> `running`
- `tool.result` -> `succeeded`
- malformed / explicit error payload -> `failed` if detectable

Future statuses may include `waiting_user`, but this ticket should not use it yet.

## Implementation Steps

1. Add up/down migrations for `agent_tool_calls`.
2. Add `model.AgentToolCall`.
3. Add repository methods:
   - `UpsertStarted`
   - `MarkSucceeded`
   - `MarkFailed`
   - `ListByRunID` if useful for tests.
4. Add `AgentToolService` wrapping repository errors.
5. Wire the service into handler construction.
6. In `ChatHandler.SendMessage`, when handling `tool.call`, persist the started audit row using event IDs and payload.
7. When handling `tool.result`, update the audit row with result/status.
8. If persistence fails, log with run and tool IDs, keep streaming.
9. Add repository/service tests where practical.

## Invariants

- Existing tool SSE events still reach the frontend.
- Assistant message parts still include `tool_call` and `tool_result`.
- Chat completion must not fail solely because audit persistence failed.
- `runs.request_id` idempotency behavior remains unchanged.
- No interaction/resume endpoint is added.

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

If the project has a migration validation command, run it and include the output.

## Acceptance Criteria

- [ ] `agent_tool_calls` migration and Go model exist.
- [ ] Current `tool.call` events create or update audit rows.
- [ ] Current `tool.result` events mark rows succeeded or failed.
- [ ] Unique key prevents duplicate rows for repeated SSE processing.
- [ ] No HITL, ask_user, resume, or new frontend UI is included.

## Regression Risks

- Migration number may conflict with concurrent work; check `apps/api/migrations/` before implementing.
- `tool_call_id` may be empty for some providers; implementation needs a deterministic fallback or logged skip.
- Persisting raw arguments/results may store large payloads; keep current payloads small.
- Handler dependency injection may require main/wiring updates.

## Out of Scope Follow-ups

- `agent_interactions`.
- Tool retry policy.
- Runtime permission checks.
- Tool trace viewer.

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

