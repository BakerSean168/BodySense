# Phase 01c: StreamEventReducer for Consultation Chat

## Goal

Introduce a frontend `StreamEventReducer` Module that consumes `StreamEvent v1` and produces chat/runtime state without changing the visible chat UI.

## Why

Current SSE handling is callback-only and mixes Markdown text streaming, structured state updates, citations, safety events, and lifecycle events. The refactor plan requires a reducer seam before the UI can safely support tool status, ask_user cards, job progress, and replay.

## Current State

- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts` parses SSE lines and dispatches callbacks through `EVENT_MAP`.
- `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts` bridges SSE callbacks to assistant-ui generator results.
- `useAssistantChatRuntime.ts` currently references `fullText` inside `onTextDelta` before `let fullText = ''` is declared.
- `AssistantChatPanel.tsx` reads assistant-ui thread state and separately derives citations/red flags/knowledge gaps from message content.
- `apps/web/src/features/consultation/types/consultation.ts` aliases event types from `@bodysense/contracts`.

## Scope

### Allowed

- Add a pure reducer Module for consultation stream events.
- Add fixture-based unit tests for current event types.
- Refactor `useAssistantChatRuntime` to use the reducer for text snapshot and structured side callbacks.
- Fix the `fullText` declaration-order issue as part of the reducer migration.
- Keep `useSSEProcessor` as the low-level SSE line parser.

### Not Allowed

- Do not implement AskUserCard.
- Do not add new event names or change contracts.
- Do not redesign `AssistantChatPanel` layout.
- Do not introduce job/tool lifecycle UI beyond preserving existing tool callbacks if present.
- Do not change backend behavior.

## Target Files

- `apps/web/src/features/consultation/runtime/streamEventReducer.ts` (new, likely)
- `apps/web/src/features/consultation/runtime/streamEventReducer.test.ts` (new, likely)
- `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`
- `apps/web/src/features/consultation/hooks/useSSEProcessor.ts` (likely, only if helper export is needed)
- `apps/web/src/features/consultation/types/consultation.ts` (likely)

## Design Notes

Suggested reducer state:

```ts
type ConsultationStreamState = {
  assistantText: string;
  conversationId?: string;
  assistantMessageId?: string;
  persistedUserMessageId?: string;
  extractedInfo: ExtractedInfo[];
  citations: Citation[];
  redFlags: RedFlagEvent | null;
  knowledgeGaps: Array<{ query: string; message: string }>;
  status: 'idle' | 'streaming' | 'completed' | 'failed';
  error?: string;
};
```

The reducer should be pure. Side effects such as `onExtractedInfoUpdate` and `pushResult` belong in the hook after comparing previous and next state or responding to returned reducer effects.

Use a small effect list if helpful:

```ts
type ReducerEffect =
  | { type: 'assistant_text_changed'; text: string }
  | { type: 'conversation_created'; conversationId: string; replacesDraftId?: string };
```

## Implementation Steps

1. Add `runtime/streamEventReducer.ts` with initial state and `reduceStreamEvent`.
2. Support all current event types in `packages/contracts/src/stream-events.ts`.
3. Treat unknown events as no-op plus optional debug entry; do not throw in production reducer.
4. Add fixtures for:
   - `message.text.delta` appending text.
   - `state.extracted_info.upsert` merging by `body_part`.
   - `source.citation.added` deduping stable citations.
   - `safety.red_flag.detected`.
   - `stream.error` and `stream.done`.
5. Update `useAssistantChatRuntime` so SSE callbacks dispatch events to the reducer.
6. Yield assistant-ui text snapshots from reducer state.
7. Keep parent callbacks (`onConversationCreated`, `onMessagePersisted`, `onExtractedInfoUpdate`, etc.) working.
8. Remove the local `fullText` mutation pattern or declare it before callbacks if a small bridge remains.

## Invariants

- Current visible chat streaming behavior remains unchanged.
- Extracted info still updates the parent ConsultationPage.
- Citation and red flag callbacks still fire.
- Malformed SSE JSON remains ignored by `useSSEProcessor`.
- No backend or contract changes are required.

## Verification Commands

```bash
pnpm nx run web:lint
pnpm nx run web:typecheck
pnpm nx run web:test
```

If `web:typecheck` or `web:test` targets are not defined in the current Nx project, run the nearest available commands and report the missing targets:

```bash
pnpm nx run web:lint
pnpm test -- --run streamEventReducer
```

## Acceptance Criteria

- [ ] A pure `StreamEventReducer` Module exists with fixture tests.
- [ ] `useAssistantChatRuntime` no longer owns ad hoc full-text accumulation as callback-local mutable state.
- [ ] The `fullText` declaration-order bug is removed.
- [ ] No AskUserCard, Job UI, or new StreamEvent names are introduced.
- [ ] Existing chat UI behavior remains the same.

## Regression Risks

- Assistant-ui generator may miss the final text snapshot if reducer effects are not flushed.
- Parent callbacks may fire multiple times if reducer effects are not deduped.
- Citation dedupe by title may hide distinct sources with the same title; preserve current behavior unless a stable ID exists.
- Tests may need local Vitest setup if no web test target exists.

## Out of Scope Follow-ups

- Message part renderer redesign.
- Tool lifecycle rows.
- AskUserCard and resume.
- Stream replay from persisted events.

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

