# Phase 02a: Python ToolRegistry and ToolExecutor Skeleton

## Goal

Introduce Python `ToolRegistry`, `ToolExecutor`, and shared tool types without migrating existing consultation tools yet.

## Why

The current consultation graph executes tools inline inside `generate_response`. Before moving `search_knowledge` or symptom extraction, the project needs a stable tool runtime seam with typed definitions, validation, status, and errors. This creates the Module interface while keeping behavior unchanged.

## Current State

- `apps/ai-service/src/services/consultation_graph.py` defines `KNOWLEDGE_SEARCH_TOOL` and builds the `extract_symptom_info` tool inline from `SYMPTOM_EXTRACTION_TOOL`.
- Tool execution is hard-coded with `if tc_name == "extract_symptom_info"` and `elif tc_name == "search_knowledge"`.
- `apps/ai-service/src/ai/types.py` has provider-level `ToolDefinition` and `ToolCall`, but no runtime metadata such as category, timeout, or auto-execute.
- `apps/ai-service/src/ai/providers/openai_compatible.py` should remain provider-only and already emits `AiStreamEvent(type="tool_call_done")`.

## Scope

### Allowed

- Add `apps/ai-service/src/services/agent/` Module files for tool types, registry, executor, and errors.
- Define runtime-level `ToolDefinition`, `ToolCallRequest`, `ToolResult`, categories, and statuses.
- Implement registration, lookup, JSON schema/Pydantic-style argument validation where practical.
- Add unit tests for unknown tool, duplicate registration, successful handler, validation failure, and handler exception.
- Provide conversion from runtime tool definitions to provider `ai.types.ToolDefinition`.

### Not Allowed

- Do not change `consultation_graph.py` behavior yet.
- Do not migrate `search_knowledge` or `extract_symptom_info` in this ticket.
- Do not add ask_user.
- Do not add Go database audit.
- Do not change provider adapter code except import-safe type reuse if unavoidable.

## Target Files

- `apps/ai-service/src/services/agent/__init__.py` (new)
- `apps/ai-service/src/services/agent/tool_types.py` (new)
- `apps/ai-service/src/services/agent/tool_registry.py` (new)
- `apps/ai-service/src/services/agent/tool_executor.py` (new)
- `apps/ai-service/src/services/agent/errors.py` (new, likely)
- `apps/ai-service/tests/unit/test_tool_registry.py` (new, likely)
- `apps/ai-service/tests/unit/test_tool_executor.py` (new, likely)

## Design Notes

Suggested statuses:

```txt
success
failed
interrupted
requires_confirmation
```

Suggested categories:

```txt
query
write
human
dangerous
```

Suggested `ToolResult` fields:

```python
tool_call_id: str
tool_name: str
status: Literal["success", "failed", "interrupted", "requires_confirmation"]
content: dict[str, Any] | str | None
error: ToolError | None
interaction_id: str | None
```

The runtime-level tool definition should have more metadata than provider-level `ToolDefinition`, but expose a small `to_provider_tool()` adapter.

## Implementation Steps

1. Create `services/agent/` package.
2. Define runtime tool types in `tool_types.py`.
3. Implement `ToolRegistry.register`, `get`, `list`, and `to_provider_tools`.
4. Reject duplicate tool names at registration time.
5. Implement `ToolExecutor.execute(tool_call)`:
   - Lookup tool by name.
   - Validate arguments against the tool schema.
   - Call the async or sync handler.
   - Wrap handler exceptions into structured failed `ToolResult`.
6. Keep validation simple and deterministic; prefer `jsonschema` only if already available. If not, implement minimal required-field/type checks and document this limitation.
7. Add unit tests for registry and executor behavior.
8. Do not import this Module from `consultation_graph.py` yet except optional smoke tests.

## Invariants

- Existing consultation streaming behavior remains unchanged.
- Existing provider-level `ToolDefinition` remains usable by `OpenAICompatibleProvider`.
- Tool handlers do not directly write user business database state.
- Unknown tool returns structured failure, not an uncaught exception.

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
uv run pytest
```

## Acceptance Criteria

- [ ] `ToolRegistry` and `ToolExecutor` Modules exist and are tested.
- [ ] Runtime tool definitions can be converted to provider tool definitions.
- [ ] Validation, unknown tool, duplicate tool, and handler exception behavior are covered.
- [ ] No existing consultation tool behavior is changed.
- [ ] No ask_user or database audit is implemented.

## Regression Risks

- Adding a dependency for schema validation may affect `uv lock`; avoid if not necessary.
- Overfitting runtime types to OpenAI-compatible providers may reduce future provider leverage.
- Tests may need async handling for both sync and async handlers.

## Out of Scope Follow-ups

- Migrating `search_knowledge`.
- Migrating `extract_symptom_info`.
- Go `agent_tool_calls` persistence.
- ask_user interrupt and resume.

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

