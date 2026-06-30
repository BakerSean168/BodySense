# Phase 02c: Migrate extract_symptom_info to ToolRuntime

## Goal

Move `extract_symptom_info` behind Python `ToolRegistry` and `ToolExecutor`, preserving current symptom extraction events and phase behavior.

## Why

After the pure query tool migration, this ticket migrates the current structured extraction tool. It is still not a database write from Python; it emits structured state back to Go and frontend through existing stream events.

## Current State

- `apps/ai-service/src/prompts/consultation.py` defines `SYMPTOM_EXTRACTION_TOOL`.
- `apps/ai-service/src/services/consultation_graph.py` adds `extract_symptom_info` to the `tools` list.
- `generate_response` handles `tc_name == "extract_symptom_info"` inline:
  - Dedupes by `body_part` within the response.
  - Appends `tc_args` to `new_symptoms`.
  - Emits `{"type": "extracted_info", "info": tc_args}`.
  - Emits `tool_result`.
  - Adds a tool message saying symptom info was extracted.
- Phase progression is determined by `_determine_phase(extracted_symptoms)`.

## Scope

### Allowed

- Register `extract_symptom_info` in the default consultation ToolRegistry.
- Move schema wiring and handler to `services/agent/tools/extract_symptom_info.py`.
- Add argument validation around required fields and known schema shape.
- Preserve current dedupe-by-body-part behavior.
- Preserve current event payloads and phase decision.

### Not Allowed

- Do not write extracted symptoms directly to the database from Python.
- Do not change the symptom schema or introduce `consultation_state` in this ticket.
- Do not implement user confirmation for extracted info.
- Do not add ask_user.
- Do not modify Go persistence behavior for extracted info unless required by existing event handling.

## Target Files

- `apps/ai-service/src/services/agent/tools/extract_symptom_info.py` (new, likely)
- `apps/ai-service/src/services/agent/tool_registry.py` (likely)
- `apps/ai-service/src/services/consultation_graph.py`
- `apps/ai-service/src/prompts/consultation.py` (likely, only if moving schema exports)
- `apps/ai-service/tests/unit/test_extract_symptom_info_tool.py` (new, likely)
- `apps/ai-service/tests/unit/test_consultation_graph.py` (likely)

## Design Notes

The handler may simply normalize and validate arguments:

```python
{
    "body_part": "...",
    "symptom_type": "...",
    "duration": "...",
    "trigger": "...",
    "severity": "..."
}
```

The orchestration layer should still own:

- Per-response dedupe.
- Emitting `extracted_info`.
- Creating the tool result message for the LLM.
- Phase calculation after the graph node.

This keeps the tool handler deep enough to validate extraction arguments but not responsible for graph state transitions.

## Implementation Steps

1. Create an `extract_symptom_info` tool module.
2. Import or move the existing `SYMPTOM_EXTRACTION_TOOL` schema without changing its public semantics.
3. Register the tool in the default consultation registry.
4. Update provider tool definition construction to use the registry.
5. Replace inline `tc_name == "extract_symptom_info"` validation and status handling with `ToolExecutor.execute(...)`.
6. Preserve dedupe by `body_part` in `generate_response` or a small orchestration helper.
7. Preserve `new_symptoms` accumulation and `writer({"type": "extracted_info", ...})`.
8. Add tests for valid extraction, missing required body part, duplicate body part, and unchanged phase behavior.

## Invariants

- Existing frontend `state.extracted_info.upsert` behavior remains unchanged.
- `consultation_sessions.extracted_info` remains updated by Go's current stream handling path.
- Phase transition from `collecting` to `ready_for_analysis` remains based on existing `_determine_phase`.
- Provider adapter remains unchanged.

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

- [ ] `extract_symptom_info` is registered through `ToolRegistry`.
- [ ] Inline extraction-specific schema construction is removed from `generate_response`.
- [ ] Existing `extracted_info` stream event shape is unchanged.
- [ ] Duplicate body parts in one response still emit once.
- [ ] No database writes, ask_user, or consultation_state migration is included.

## Regression Risks

- Schema validation may reject arguments the current model commonly emits.
- Moving schema exports may break prompt imports.
- Dedupe may accidentally become global across the whole session instead of per response.
- Tool result message content changes may affect LLM follow-up text.

## Out of Scope Follow-ups

- User-confirmed symptom cards.
- Long-term health profile merge.
- `save_extracted_info` write tool.
- `consultation_state` revisioning.

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

