# Phase 03c: AskUserCard UI

## Goal

Add frontend rendering and submission for pending `ask_user` interactions using the resume endpoint from Phase 03b.

## Why

After the backend can persist and resume interactions, the user needs a stable UI to answer Agent follow-up questions. This ticket completes the narrow HITL loop without adding new tools or job behavior.

## Current State

- `AssistantChatPanel.tsx` renders assistant-ui text messages, citation chips, knowledge gap notices, and red flag banner.
- `useAssistantChatRuntime.ts` consumes SSE and forwards structured callbacks.
- Phase 01c should introduce `StreamEventReducer`.
- Phase 03b should expose a resume endpoint and public interaction event.
- There is no AskUserCard or interaction state in frontend types today.

## Scope

### Allowed

- Extend frontend contracts/types for `state.interaction.required`, `state.interaction.answered`, and/or `stream.interrupted` as implemented in Phase 03b.
- Extend `StreamEventReducer` to store pending interactions.
- Add `AskUserCard` for `text`, `single_choice`, `multi_choice`, `number`, `date`, and `file` placeholders as practical.
- Add resume call in consultation service.
- Update chat UI to render pending/answered/cancelled interaction state.
- Add tests for reducer interaction events and AskUserCard basic submission.

### Not Allowed

- Do not change backend route shape except consuming what Phase 03b provides.
- Do not implement confirm_action or high-risk approvals.
- Do not implement file upload resume fully unless Phase 03b supports it; show disabled placeholder if not supported.
- Do not redesign the whole chat UI.
- Do not introduce Job UI.

## Target Files

- `apps/web/src/features/consultation/components/AskUserCard.tsx` (new, likely)
- `apps/web/src/features/consultation/runtime/streamEventReducer.ts`
- `apps/web/src/features/consultation/runtime/streamEventReducer.test.ts`
- `apps/web/src/features/consultation/hooks/useAssistantChatRuntime.ts`
- `apps/web/src/features/consultation/components/AssistantChatPanel.tsx`
- `apps/web/src/features/consultation/services/consultationService.ts`
- `apps/web/src/features/consultation/types/consultation.ts`
- `packages/contracts/src/stream-events.ts` (likely, only if Phase 03b did not already update it)

## Design Notes

`AskUserCard` props:

```ts
type AskUserCardProps = {
  interactionId: string;
  conversationId: string;
  payload: {
    question: string;
    reason?: string;
    answer_type: 'text' | 'single_choice' | 'multi_choice' | 'number' | 'date' | 'file';
    options?: string[];
    required: boolean;
  };
  status: 'pending' | 'submitting' | 'answered' | 'failed' | 'cancelled';
  onSubmit(answer: unknown): Promise<void>;
};
```

UI behavior:

- Pending card appears inline in the assistant message area.
- After submit, card shows answered state and disables controls.
- Runtime resumes streaming through the same assistant-ui flow if backend returns SSE.
- If resume fails, card shows error and allows retry.

## Implementation Steps

1. Add or extend frontend interaction event types.
2. Extend `StreamEventReducer` with `pendingInteractions` keyed by `interaction_id`.
3. Add reducer tests for required/answered events.
4. Add `consultationService.resumeInteraction(...)`.
5. Create `AskUserCard` with controlled answer state and validation.
6. Integrate AskUserCard into `AssistantChatPanel` or a message part renderer with minimal layout changes.
7. Update `useAssistantChatRuntime` to handle interrupted stream state cleanly.
8. When submitting an answer, call resume endpoint and consume returned SSE if the endpoint streams continuation.
9. Add user-facing error state for failed resume.

## Invariants

- Existing normal chat streaming remains unchanged.
- Pending interactions survive in reducer state until answered/cancelled.
- Markdown renderer only renders text parts; AskUserCard is structured UI.
- Submit is disabled while an answer is being sent.
- Duplicate submit should not create duplicate resume requests.

## Verification Commands

```bash
pnpm nx run web:lint
pnpm nx run web:typecheck
pnpm nx run web:test
```

Fallback:

```bash
pnpm nx run web:lint
pnpm test -- --run AskUserCard
```

## Acceptance Criteria

- [ ] `AskUserCard` renders supported answer types.
- [ ] Interaction events are represented in reducer state.
- [ ] Submitting answer calls the Phase 03b resume endpoint.
- [ ] Answered state is visible and controls are disabled after success.
- [ ] Existing chat, citations, extracted info, and red flag behavior still work.

## Regression Risks

- assistant-ui may not naturally support non-text message parts; structured card state may need to live outside message content initially.
- Resume streaming could conflict with existing `isRunning` checks.
- Multi-choice answer serialization must match backend expectations.
- File answer type may need a later upload integration.

## Out of Scope Follow-ups

- ConfirmActionCard.
- RequestUploadCard.
- Interaction list after page refresh if no endpoint exists yet.
- Full trace/debug UI.

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

