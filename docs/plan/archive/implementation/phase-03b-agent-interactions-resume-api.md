# Phase 03b: agent_interactions and Resume API

> **⚠️ SUPERSEDED — 本方案从未实施，已被 ADR 0002 完全取代。**
>
> 本 Phase 设计的 Go 侧 `agent_interactions` 表和 resume API 未被创建。当前架构中 interrupt/resume 由 Python LangGraph 原生 `interrupt()` / `Command(resume=...)` 处理，Go 仅代理到 Python runtime API。详见 [`docs/adr/0002-agent-runtime-ownership.md`](../../../adr/0002-agent-runtime-ownership.md) 和 [`docs/plan/active/final-agent-runtime-architecture.md`](../../active/final-agent-runtime-architecture.md)。

## Goal

Add Go-side `agent_interactions` persistence and a resume endpoint for `ask_user` interruptions, without implementing the frontend card.

## Why

HITL tools require a durable pause/resume model. This ticket makes Go the source of truth for pending interactions and run `waiting_user` state, so the frontend can refresh safely and Python can continue after the user answers.

## Current State

- Phase 03a should define Python `ask_user` as an interrupted tool result.
- Go currently has no `agent_interactions` table.
- `runs.status` currently supports `running/completed/failed/cancelled` in comments and model field.
- `ChatHandler` currently treats streams as complete, failed, or aborted; no `stream.interrupted` handling exists.
- No resume route exists in Go or Python.

## Scope

### Allowed

- Add `agent_interactions` migration, model, repository, and service.
- Extend run status semantics to include `waiting_user`.
- Add nullable `interaction_id` to Go/Python/TS StreamEvent IDs if needed by the event contract.
- Add Go endpoint: `POST /api/v1/consultation/:conversationId/interactions/:interactionId/resume` or a route matching existing router style.
- Persist pending interaction when a Python interruption event is received.
- Mark interaction answered on resume with idempotent behavior.
- Forward resume payload to a Python resume endpoint stub or defined interface if available.

### Not Allowed

- Do not implement AskUserCard UI.
- Do not introduce JobRuntime.
- Do not implement multiple simultaneous pending interactions.
- Do not add dangerous tool approval flows.
- Do not let Python directly mutate user-owned interaction rows.

## Target Files

- `apps/api/migrations/000016_create_agent_interactions.up.sql` (new, likely)
- `apps/api/migrations/000016_create_agent_interactions.down.sql` (new, likely)
- `apps/api/internal/model/agent_interaction.go` (new, likely)
- `apps/api/internal/repository/agent_interaction_repository.go` (new, likely)
- `apps/api/internal/service/agent_interaction_service.go` (new, likely)
- `apps/api/internal/service/run_service.go`
- `apps/api/internal/repository/run_repository.go`
- `apps/api/internal/handler/chat_handler.go`
- `apps/api/internal/handler/consultation_handler.go` or route wiring file (likely)
- `apps/api/internal/dto/stream_event.go`
- `apps/ai-service/src/api/routes/chat.py` (likely, resume endpoint stub/interface)
- `apps/ai-service/src/models/stream_event.py` (likely)
- `packages/contracts/src/stream-events.ts`
- `packages/contracts/schemas/stream-event.v1.schema.json`

## Design Notes

Suggested `agent_interactions` table:

```sql
agent_interactions (
  id UUID PRIMARY KEY DEFAULT uuidv7(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  tool_call_id TEXT NOT NULL,
  type VARCHAR(50) NOT NULL,
  status VARCHAR(30) NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  user_response JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  answered_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  metadata JSONB NOT NULL DEFAULT '{}'
)
```

Statuses:

```txt
pending
answered
cancelled
expired
```

Resume must be idempotent:

- If pending: store answer and continue.
- If answered with same idempotency key: return existing accepted status.
- If answered with different payload: return conflict.

## Implementation Steps

1. Add migration and model for `agent_interactions`.
2. Add repository/service methods:
   - `CreatePending`
   - `GetPendingForUserConversation`
   - `Answer`
   - `Cancel`
3. Extend `RunService` with `MarkWaitingUser` and `ResumeRunning` methods.
4. Extend StreamEvent IDs with `interaction_id` across Go, Python, TS contracts.
5. Add handling for a Python interruption event, likely `state.interaction.required` or `stream.interrupted`.
6. On interruption:
   - Persist interaction.
   - Mark run `waiting_user`.
   - Emit public interruption event.
   - End SSE without marking the assistant message completed as a normal final answer unless a partial message is valid.
7. Add resume route with auth and ownership checks.
8. Define Go -> Python resume request DTO.
9. Add a Python resume route stub/interface that can accept the tool result and return stream events, or clearly return not implemented if Phase 03b is scoped to Go persistence only. Prefer a minimal working path if Phase 03a has Python support.
10. Add tests for interaction creation, ownership checks, and idempotent answer behavior.

## Invariants

- Existing non-interrupted chat flow remains unchanged.
- A pending interaction belongs to one user-owned conversation.
- A run in `waiting_user` is not considered failed.
- Repeating resume must not duplicate user answers.
- No frontend UI is required to manually test the endpoint.

## Verification Commands

```bash
pnpm nx run api:lint
pnpm nx run api:test
pnpm nx run ai-service:lint
pnpm nx run ai-service:test
pnpm nx run contracts:typecheck
```

Fallback:

```bash
cd apps/api
go vet ./...
go test ./...
cd ../ai-service
uv run ruff check .
uv run pytest
```

## Acceptance Criteria

- [ ] `agent_interactions` table/model/service exists.
- [ ] Run can enter `waiting_user`.
- [ ] Interruption events create pending interaction rows.
- [ ] Resume endpoint validates ownership and stores answer idempotently.
- [ ] Existing normal chat flow still completes.
- [ ] No AskUserCard UI or JobRuntime code is included.

## Regression Risks

- Incorrect run completion logic may mark interrupted runs as failed or completed.
- Stream termination on interruption may leave assistant messages in an inconsistent status.
- Route shape may conflict with existing consultation route naming.
- Python resume may need more graph state than is persisted; document any limitation clearly.

## Out of Scope Follow-ups

- Frontend AskUserCard.
- Multiple concurrent interactions.
- Confirm-action approval flow.
- Full LangGraph checkpoint persistence.

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

