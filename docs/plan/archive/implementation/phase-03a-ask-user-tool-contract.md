# Phase 03a: ask_user Tool Contract

## Goal

Define and register the Python `ask_user` tool contract and stream interruption event shape, without implementing Go resume persistence or frontend UI.

## Why

`ask_user` is a HITL tool and must be designed as an interrupt, not a synchronous function. This ticket establishes the tool schema and Python-side interrupted `ToolResult` shape before adding `agent_interactions` and resume APIs.

## Current State

- Existing tools are `search_knowledge` and `extract_symptom_info`.
- `ToolResult` skeleton should exist from Phase 02a.
- There is no `ask_user` schema, no `agent_interactions`, and no `waiting_user` run state.
- `StreamEventIds` does not yet include `interaction_id`.
- Frontend has no AskUserCard.

## Scope

### Allowed

- Add `ask_user` tool schema and register it as category `human`.
- Define `AskUserPayload` and `ToolResult(status="interrupted")`.
- Add Python tests proving `ask_user` never blocks and returns interrupted.
- Add a Python internal event mapping shape for `state.interaction.required` or `stream.interrupted`, but do not require Go to persist it yet.
- Document prompt/tool policy limiting ask_user usage.

### Not Allowed

- Do not add Go `agent_interactions` table.
- Do not add resume endpoint.
- Do not update frontend UI.
- Do not switch run status to `waiting_user`.
- Do not allow the tool to call `input()` or wait for user response.
- Do not make `ask_user` available in production prompt flow unless a feature flag or explicit graph test path prevents unhandled interrupts.

## Target Files

- `apps/ai-service/src/services/agent/tools/ask_user.py` (new, likely)
- `apps/ai-service/src/services/agent/tool_types.py` (likely)
- `apps/ai-service/src/services/agent/tool_registry.py` (likely)
- `apps/ai-service/src/services/agent/prompts.py` (new, likely)
- `apps/ai-service/tests/unit/test_ask_user_tool.py` (new, likely)
- `apps/ai-service/src/models/stream_event.py` (likely, only if adding internal type support)

## Design Notes

Tool schema:

```json
{
  "question": "string",
  "reason": "string",
  "answer_type": "text | single_choice | multi_choice | number | date | file",
  "options": ["string"],
  "required": true
}
```

Runtime result:

```python
ToolResult(
    tool_call_id=...,
    tool_name="ask_user",
    status="interrupted",
    content={
        "type": "ask_user",
        "payload": {...}
    },
)
```

Policy:

- Ask only when missing information is necessary.
- Do not ask about already provided information.
- One ask_user per run until Phase 03b defines resume.
- Question length should be capped, e.g. 80 Chinese characters.

## Implementation Steps

1. Add `ask_user.py` under `services/agent/tools/`.
2. Define schema and a handler that validates payload and returns interrupted immediately.
3. Register `ask_user` in a registry factory only behind an explicit flag or non-default option until Go/frontend support exists.
4. Add policy text for future developer prompt usage.
5. Add tests:
   - Valid single-choice payload returns interrupted.
   - Missing question fails validation.
   - Invalid answer_type fails validation.
   - Handler does not block.
6. If event helper types are added, keep them internal and do not change public SSE contract yet.

## Invariants

- Existing consultation flow does not start calling ask_user in normal operation.
- No user response is required by this ticket.
- Tool execution remains deterministic and non-blocking.
- No database state changes are introduced.

## Verification Commands

```bash
pnpm nx run ai-service:lint
pnpm nx run ai-service:typecheck
pnpm nx run ai-service:test
```

Fallback:

```bash
cd apps/ai-service
uv run ruff check .
uv run pyright src
uv run pytest tests/unit/test_ask_user_tool.py
```

## Acceptance Criteria

- [ ] `ask_user` schema and handler exist.
- [ ] Handler returns `ToolResult.status == "interrupted"` and never waits synchronously.
- [ ] Tests cover valid and invalid payloads.
- [ ] Normal consultation flow is not exposed to unsupported ask_user interruptions.
- [ ] No Go persistence, resume endpoint, or frontend UI is implemented.

## Regression Risks

- Registering ask_user too early may cause the model to call it and break current chat.
- Overly broad schema may allow unusable frontend payloads.
- Prompt policy may be ignored if not enforced by runtime caps in later tickets.

## Out of Scope Follow-ups

- `agent_interactions` persistence.
- Resume API.
- AskUserCard.
- Run `waiting_user` status.

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

