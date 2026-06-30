# Phase 02b: Migrate search_knowledge to ToolRuntime

## Goal

Move the existing `search_knowledge` tool implementation behind Python `ToolRegistry` and `ToolExecutor` while preserving current RAG behavior and SSE events.

## Why

`search_knowledge` is the safest first tool migration because it is a pure query tool. Migrating it proves the ToolRuntime seam without introducing writes, HITL pauses, or Go persistence.

## Current State

- `apps/ai-service/src/services/consultation_graph.py` defines `KNOWLEDGE_SEARCH_TOOL`.
- `execute_search_knowledge(arguments)` calls `get_knowledge_library().search(...)`.
- `generate_response` handles `tc_name == "search_knowledge"` inline, dedupes repeated queries, emits citation or knowledge gap events, emits `tool_result`, and appends a tool message.
- `apps/ai-service/src/rag/knowledge_library.py` owns knowledge search.
- `apps/ai-service/src/ai/providers/openai_compatible.py` only emits completed tool calls and should remain unchanged.

## Scope

### Allowed

- Register `search_knowledge` in the new ToolRegistry.
- Move the search handler into `services/agent/tools/search_knowledge.py` or equivalent.
- Keep query dedupe in the orchestration layer, not the handler, unless the handler receives a run-local context.
- Keep existing event payloads: `tool_call`, `tool_result`, `citation`, `knowledge_gap`.
- Add unit tests for successful results, no results, and repeated-query behavior if currently covered by graph tests.

### Not Allowed

- Do not change retrieval ranking, filters, embeddings, or knowledge lifecycle status.
- Do not add published-only filtering; that belongs to Phase 07a.
- Do not add Go `agent_tool_calls` persistence; that belongs to Phase 02d.
- Do not migrate `extract_symptom_info` in this ticket.
- Do not add ask_user.

## Target Files

- `apps/ai-service/src/services/agent/tools/search_knowledge.py` (new, likely)
- `apps/ai-service/src/services/agent/tool_registry.py` (likely)
- `apps/ai-service/src/services/agent/__init__.py` (likely)
- `apps/ai-service/src/services/consultation_graph.py`
- `apps/ai-service/tests/unit/test_consultation_graph.py` (likely)
- `apps/ai-service/tests/unit/test_search_knowledge_tool.py` (new, likely)

## Design Notes

The handler should return structured content that the graph can map to:

```python
{
    "result_text": "...",
    "has_results": True,
    "raw_results": [...],
}
```

The graph or an `AgentOrchestrator` adapter remains responsible for turning raw results into citation events because SSE event emission is orchestration behavior, not tool business logic.

Do not put frontend event names into the tool handler interface. The handler should expose semantic results; the graph maps them.

## Implementation Steps

1. Create a search knowledge tool module under `services/agent/tools/`.
2. Move the schema currently named `KNOWLEDGE_SEARCH_TOOL` into the tool module.
3. Move or wrap `execute_search_knowledge` as the registered handler.
4. Register the tool in a default consultation registry factory.
5. Update `consultation_graph.generate_response` to get provider tool definitions from the registry.
6. In the tool-call loop, for `search_knowledge`, call `ToolExecutor.execute(...)`.
7. Preserve existing repeated query dedupe behavior.
8. Preserve existing citation and knowledge gap event emission.
9. Preserve existing tool message content fed back to the LLM.
10. Add or update tests to prove current `search_knowledge` stream events still appear.

## Invariants

- The same user prompt still produces a knowledge search tool definition available to the LLM.
- Existing citation event shape remains unchanged.
- Existing no-result knowledge gap behavior remains unchanged.
- Provider adapter remains thin and unchanged.
- No database writes are introduced by the tool.

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
uv run pytest tests/unit/test_consultation_graph.py tests/unit/test_chat_service.py
```

## Acceptance Criteria

- [ ] `search_knowledge` is registered through `ToolRegistry`.
- [ ] `generate_response` no longer calls the old inline `execute_search_knowledge` function directly.
- [ ] Citation and knowledge gap events remain compatible with current frontend consumption.
- [ ] Query dedupe behavior is preserved.
- [ ] No ask_user, audit table, or lifecycle filtering is included.

## Regression Risks

- Tool result content may change and affect the next LLM round.
- Raw search result objects may not be serializable if passed too far.
- Dedupe moving into the wrong layer may suppress valid searches across rounds.
- Tests may need fakes for `KnowledgeLibrary.search`.

## Out of Scope Follow-ups

- Published/reviewed search filtering.
- Knowledge citation stable IDs.
- Tool call audit persistence.
- Search result reranking changes.

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

